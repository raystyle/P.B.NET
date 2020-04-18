package msfrpc

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/xpanic"
)

// SessionList is used to list all active sessions in the framework instance.
func (msf *MSFRPC) SessionList(ctx context.Context) (map[uint64]*SessionInfo, error) {
	request := SessionListRequest{
		Method: MethodSessionList,
		Token:  msf.GetToken(),
	}
	var (
		result   map[uint64]*SessionInfo
		msfError MSFError
	)
	err := msf.sendWithReplace(ctx, &request, &result, &msfError)
	if err != nil {
		return nil, err
	}
	if msfError.Err {
		if msfError.ErrorMessage == ErrInvalidToken {
			msfError.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&msfError)
	}
	return result, nil
}

// SessionStop is used to terminate the session specified in the SessionID parameter.
func (msf *MSFRPC) SessionStop(ctx context.Context, id uint64) error {
	request := SessionStopRequest{
		Method: MethodSessionStop,
		Token:  msf.GetToken(),
		ID:     id,
	}
	var result SessionStopResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "Unknown Session ID":
			id := strconv.FormatUint(id, 10)
			result.ErrorMessage = ErrUnknownSessionIDPrefix + id
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// SessionRead is used to provide the ability to read output from a shell session. As
// of version 3.7.0, shell sessions also ring buffer their output, allowing multiple
// callers to read from one session without losing data. This is implemented through
// the optional ReadPointer parameter. If this parameter is not given (or set to 0),
// the server will reply with all buffered data and a new ReadPointer (as the "seq"
// element of the reply). If the caller passes this ReadPointer into subsequent calls
// to shell.read, only data since the previous read will be returned. By continuing
// to track the ReadPointer returned by the last call and pass it into the next call,
// multiple readers can all follow the output from a single session without conflict.
func (msf *MSFRPC) SessionRead(
	ctx context.Context,
	id uint64,
	pointer uint64,
) (*SessionShellReadResult, error) {
	request := SessionShellReadRequest{
		Method:  MethodSessionShellRead,
		Token:   msf.GetToken(),
		ID:      id,
		Pointer: pointer,
	}
	var result SessionShellReadResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		id := strconv.FormatUint(id, 10)
		switch result.ErrorMessage {
		case ErrUnknownSessionID + id:
			result.ErrorMessage = ErrUnknownSessionIDPrefix + id
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return &result, nil
}

// SessionWrite is used to provide the ability to write data into an active shell session.
// Most sessions require a terminating newline before they will process a command.
func (msf *MSFRPC) SessionWrite(ctx context.Context, id uint64, data string) (uint64, error) {
	if len(data) == 0 {
		return 0, nil
	}
	request := SessionShellWriteRequest{
		Method: MethodSessionShellWrite,
		Token:  msf.GetToken(),
		ID:     id,
		Data:   data,
	}
	var result SessionShellWriteResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return 0, err
	}
	if result.Err {
		id := strconv.FormatUint(id, 10)
		switch result.ErrorMessage {
		case ErrUnknownSessionID + id:
			result.ErrorMessage = ErrUnknownSessionIDPrefix + id
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return 0, errors.WithStack(&result.MSFError)
	}
	n, _ := strconv.ParseUint(result.WriteCount, 10, 64)
	return n, nil
}

// SessionUpgrade is used to attempt to spawn a new Meterpreter session through an existing
// shell session. This requires that a multi/handler be running (windows/meterpreter/reverse_tcp)
// and that the host and port of this handler is provided to this method.
//
// API from MSFRPC will leaks, so we do it self.
func (msf *MSFRPC) SessionUpgrade(
	ctx context.Context,
	id uint64,
	host string,
	port uint64,
	opts map[string]interface{},
	wait time.Duration,
) (*ModuleExecuteResult, error) {
	// get operating system
	sessions, err := msf.SessionList(ctx)
	if err != nil {
		return nil, err
	}
	info := sessions[id]
	if info == nil {
		return nil, errors.Errorf("invalid session id: %d", id)
	}
	os := strings.Split(info.ViaPayload, "/")[1]
	// run post module
	const module = "multi/manage/shell_to_meterpreter"
	if opts == nil {
		opts = make(map[string]interface{})
	}
	opts["SESSION"] = id
	opts["LHOST"] = host
	opts["LPORT"] = port
	opts["HANDLER"] = false
	result, err := msf.ModuleExecute(ctx, "post", module, opts)
	if err != nil {
		return nil, err
	}
	// must wait some for payload
	const minWaitTime = 5 * time.Second
	if wait < minWaitTime {
		wait = minWaitTime
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return nil, errors.WithStack(ctx.Err())
	}
	// must input some command, power shell will start work
	if os == "windows" {
		_, err = msf.SessionWrite(ctx, id, "\nwhoami\n")
	}
	// wait some time for power shell
	timer.Reset(3 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return nil, errors.WithStack(ctx.Err())
	}
	return result, err
}

// SessionMeterpreterRead is used to provide the ability to read pending output from a
// Meterpreter session console. As noted in the session.meterpreter_write documentation,
// this method is problematic when it comes to concurrent access by multiple callers and
// Post modules or Scripts should be used instead.
func (msf *MSFRPC) SessionMeterpreterRead(ctx context.Context, id uint64) (string, error) {
	request := SessionMeterpreterReadRequest{
		Method: MethodSessionMeterpreterRead,
		Token:  msf.GetToken(),
		ID:     id,
	}
	var result SessionMeterpreterReadResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return "", err
	}
	if result.Err {
		id := strconv.FormatUint(id, 10)
		switch result.ErrorMessage {
		case ErrUnknownSessionID + id:
			result.ErrorMessage = ErrUnknownSessionIDPrefix + id
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return "", errors.WithStack(&result.MSFError)
	}
	return result.Data, nil
}

// SessionMeterpreterWrite is used to provide the ability write commands into the
// Meterpreter Console. This emulates how a user would interact with a Meterpreter
// session from the Metasploit Framework Console. Note that multiple concurrent
// callers writing and reading to the same Meterpreter session through this method
// can lead to a conflict, where one caller gets the others output and vice versa.
// Concurrent access to a Meterpreter session is best handled by running Post modules
// or Scripts. A newline does not need to be specified unless the console is currently
// tied to an interactive channel, such as a sub-shell.
func (msf *MSFRPC) SessionMeterpreterWrite(ctx context.Context, id uint64, data string) error {
	if len(data) == 0 {
		return nil
	}
	request := SessionMeterpreterWriteRequest{
		Method: MethodSessionMeterpreterWrite,
		Token:  msf.GetToken(),
		ID:     id,
		Data:   data,
	}
	var result SessionMeterpreterWriteResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		id := strconv.FormatUint(id, 10)
		switch result.ErrorMessage {
		case ErrUnknownSessionID + id:
			result.ErrorMessage = ErrUnknownSessionIDPrefix + id
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// SessionMeterpreterDetach is used to stop any current channel or sub-shell interaction
// taking place by the console associated with the specified Meterpreter session. This
// simulates the console user pressing the Control+Z hotkey.
func (msf *MSFRPC) SessionMeterpreterDetach(ctx context.Context, id uint64) error {
	request := SessionMeterpreterSessionDetachRequest{
		Method: MethodSessionMeterpreterSessionDetach,
		Token:  msf.GetToken(),
		ID:     id,
	}
	var result SessionMeterpreterSessionDetachResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		id := strconv.FormatUint(id, 10)
		switch result.ErrorMessage {
		case ErrUnknownSessionID + id:
			result.ErrorMessage = ErrUnknownSessionIDPrefix + id
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// SessionMeterpreterKill is used to terminate the current channel or sub-shell that
// the console associated with the specified Meterpreter session is interacting with.
// This simulates the console user pressing the Control+C hotkey.
func (msf *MSFRPC) SessionMeterpreterKill(ctx context.Context, id uint64) error {
	request := SessionMeterpreterSessionKillRequest{
		Method: MethodSessionMeterpreterSessionKill,
		Token:  msf.GetToken(),
		ID:     id,
	}
	var result SessionMeterpreterSessionKillResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		id := strconv.FormatUint(id, 10)
		switch result.ErrorMessage {
		case ErrUnknownSessionID + id:
			result.ErrorMessage = ErrUnknownSessionIDPrefix + id
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// SessionMeterpreterRunSingle is used to provide the ability to run a single Meterpreter
// console command. This method does not need be terminated by a newline. The advantage
// to session.meterpreter_run_single over session.meterpreter_write is that this method
// will always run the Meterpreter command, even if the console tied to this sessions is
// interacting with a channel.
func (msf *MSFRPC) SessionMeterpreterRunSingle(ctx context.Context, id uint64, cmd string) error {
	request := SessionMeterpreterRunSingleRequest{
		Method:  MethodSessionMeterpreterRunSingle,
		Token:   msf.GetToken(),
		ID:      id,
		Command: cmd,
	}
	var result SessionMeterpreterRunSingleResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		id := strconv.FormatUint(id, 10)
		switch result.ErrorMessage {
		case ErrUnknownSessionID + id:
			result.ErrorMessage = ErrUnknownSessionIDPrefix + id
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// SessionCompatibleModules is used to return a list of Post modules that are compatible
// with the specified session. This includes matching Meterpreter Post modules to Meterpreter
// sessions and enforcing platform and architecture restrictions.
func (msf *MSFRPC) SessionCompatibleModules(ctx context.Context, id uint64) ([]string, error) {
	request := SessionCompatibleModulesRequest{
		Method: MethodSessionCompatibleModules,
		Token:  msf.GetToken(),
		ID:     id,
	}
	var result SessionCompatibleModulesResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		if result.ErrorMessage == ErrInvalidToken {
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Modules, nil
}

// Shell is used to provide a more gracefully io. It implemented io.ReadWriteCloser.
type Shell struct {
	ctx *MSFRPC

	id       uint64
	interval time.Duration

	logSrc  string
	pr      *io.PipeReader
	pw      *io.PipeWriter
	writeMu sync.Mutex
	token   chan struct{}

	context   context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewShell is used to create a graceful IO stream with shell id.
// If appear some errors about network, you can use it to attach an exist shell session.
func (msf *MSFRPC) NewShell(id uint64, interval time.Duration) *Shell {
	if interval < minReadInterval {
		interval = minReadInterval
	}
	shell := Shell{
		ctx:      msf,
		id:       id,
		interval: interval,
		logSrc:   fmt.Sprintf("msfrpc-shell-%d", id),
		token:    make(chan struct{}),
	}
	shell.pr, shell.pw = io.Pipe()
	shell.context, shell.cancel = context.WithCancel(context.Background())
	shell.wg.Add(2)
	go shell.reader()
	go shell.writeLimiter()
	return &shell
}

func (shell *Shell) log(lv logger.Level, log ...interface{}) {
	shell.ctx.logger.Println(lv, shell.logSrc, log...)
}

func (shell *Shell) reader() {
	defer func() {
		if r := recover(); r != nil {
			shell.log(logger.Fatal, xpanic.Print(r, "Shell.reader"))
			// restart reader
			time.Sleep(time.Second)
			go shell.reader()
		} else {
			shell.close()
			shell.wg.Done()
		}
	}()
	if !shell.ctx.trackShell(shell, true) {
		return
	}
	defer shell.ctx.trackShell(shell, false)
	// don't use ticker otherwise read write will appear confusion.
	timer := time.NewTimer(shell.interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			if !shell.read() {
				return
			}
		case <-shell.context.Done():
			return
		}
		timer.Reset(shell.interval)
	}
}

func (shell *Shell) read() bool {
	result, err := shell.ctx.SessionRead(shell.context, shell.id, 0)
	if err != nil {
		return false
	}
	if len(result.Data) == 0 {
		return true
	}
	shell.writeMu.Lock()
	defer shell.writeMu.Unlock()
	_, err = shell.pw.Write([]byte(result.Data))
	return err == nil
}

func (shell *Shell) writeLimiter() {
	defer func() {
		if r := recover(); r != nil {
			shell.log(logger.Fatal, xpanic.Print(r, "Shell.writeLimiter"))
			// restart limiter
			time.Sleep(time.Second)
			go shell.writeLimiter()
		} else {
			close(shell.token)
			shell.wg.Done()
		}
	}()
	// don't use ticker otherwise read write will appear confusion.
	interval := 2 * shell.interval
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			select {
			case shell.token <- struct{}{}:
			case <-shell.context.Done():
				return
			}
		case <-shell.context.Done():
			return
		}
		timer.Reset(interval)
	}
}

// Read is used to read output from shell session.
func (shell *Shell) Read(b []byte) (int, error) {
	return shell.pr.Read(b)
}

// Write is used to write command to shell session, it need add "\r\n".
func (shell *Shell) Write(b []byte) (int, error) {
	select {
	case <-shell.token:
	case <-shell.context.Done():
		return 0, shell.context.Err()
	}
	shell.writeMu.Lock()
	defer shell.writeMu.Unlock()
	n, err := shell.ctx.SessionWrite(shell.context, shell.id, string(b))
	return int(n), err
}

// CompatibleModules is used to return a list of Post modules that compatible.
func (shell *Shell) CompatibleModules(ctx context.Context) ([]string, error) {
	return shell.ctx.SessionCompatibleModules(ctx, shell.id)
}

// Close is used to close reader, it will not kill the shell session.
func (shell *Shell) Close() error {
	shell.destroy(true)
	return nil
}

func (shell *Shell) closeNotWait() {
	shell.destroy(false)
}

func (shell *Shell) destroy(wait bool) {
	shell.close()
	if wait {
		shell.wg.Wait()
	}
}

func (shell *Shell) close() {
	shell.closeOnce.Do(func() {
		shell.cancel()
		_ = shell.pw.Close()
		_ = shell.pr.Close()
	})
}

// Kill is used to kill shell session.
func (shell *Shell) Kill() error {
	err := shell.ctx.SessionStop(shell.context, shell.id)
	if err != nil {
		return err
	}
	return shell.Close()
}

// Meterpreter is used to provide a more gracefully io. It implemented io.ReadWriteCloser.
type Meterpreter struct {
	ctx *MSFRPC

	id       uint64
	interval time.Duration

	logSrc  string
	pr      *io.PipeReader
	pw      *io.PipeWriter
	writeMu sync.Mutex
	token   chan struct{}

	context   context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewMeterpreter is used to create a graceful IO stream with meterpreter id.
// If appear some errors about network, you can use it to attach an exist meterpreter session.
func (msf *MSFRPC) NewMeterpreter(id uint64, interval time.Duration) *Meterpreter {
	if interval < minReadInterval {
		interval = minReadInterval
	}
	meterpreter := Meterpreter{
		ctx:      msf,
		id:       id,
		interval: interval,
		logSrc:   fmt.Sprintf("msfrpc-meterpreter-%d", id),
		token:    make(chan struct{}),
	}
	meterpreter.pr, meterpreter.pw = io.Pipe()
	meterpreter.context, meterpreter.cancel = context.WithCancel(context.Background())
	meterpreter.wg.Add(2)
	go meterpreter.reader()
	go meterpreter.writeLimiter()
	return &meterpreter
}

func (mp *Meterpreter) log(lv logger.Level, log ...interface{}) {
	mp.ctx.logger.Println(lv, mp.logSrc, log...)
}

func (mp *Meterpreter) reader() {
	defer func() {
		if r := recover(); r != nil {
			mp.log(logger.Fatal, xpanic.Print(r, "Meterpreter.reader"))
			// restart reader
			time.Sleep(time.Second)
			go mp.reader()
		} else {
			mp.close()
			mp.wg.Done()
		}
	}()
	if !mp.ctx.trackMeterpreter(mp, true) {
		return
	}
	defer mp.ctx.trackMeterpreter(mp, false)
	// don't use ticker otherwise read write will appear confusion.
	timer := time.NewTimer(mp.interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			if !mp.read() {
				return
			}
		case <-mp.context.Done():
			return
		}
		timer.Reset(mp.interval)
	}
}

func (mp *Meterpreter) read() bool {
	data, err := mp.ctx.SessionMeterpreterRead(mp.context, mp.id)
	if err != nil {
		return false
	}
	if len(data) == 0 {
		return true
	}
	mp.writeMu.Lock()
	defer mp.writeMu.Unlock()
	_, err = mp.pw.Write([]byte(data))
	return err == nil
}

func (mp *Meterpreter) writeLimiter() {
	defer func() {
		if r := recover(); r != nil {
			mp.log(logger.Fatal, xpanic.Print(r, "Meterpreter.writeLimiter"))
			// restart limiter
			time.Sleep(time.Second)
			go mp.writeLimiter()
		} else {
			close(mp.token)
			mp.wg.Done()
		}
	}()
	// don't use ticker otherwise read write will appear confusion.
	interval := 2 * mp.interval
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			select {
			case mp.token <- struct{}{}:
			case <-mp.context.Done():
				return
			}
		case <-mp.context.Done():
			return
		}
		timer.Reset(interval)
	}
}

// Read is used to read output form meterpreter session.
func (mp *Meterpreter) Read(b []byte) (int, error) {
	return mp.pr.Read(b)
}

// Write is used to write command to meterpreter session, it don't need add "\r\n".
func (mp *Meterpreter) Write(b []byte) (int, error) {
	select {
	case <-mp.token:
	case <-mp.context.Done():
		return 0, mp.context.Err()
	}
	mp.writeMu.Lock()
	defer mp.writeMu.Unlock()
	err := mp.ctx.SessionMeterpreterWrite(mp.context, mp.id, string(b))
	return len(b), err
}

// Detach is used to detach current meterpreter session.
func (mp *Meterpreter) Detach(ctx context.Context) error {
	err := mp.ctx.SessionMeterpreterDetach(ctx, mp.id)
	if err != nil {
		return err
	}
	_, err = mp.pw.Write([]byte("\r\n\r\n"))
	return err
}

// Interrupt is used to send interrupt to current meterpreter session.
func (mp *Meterpreter) Interrupt(ctx context.Context) error {
	err := mp.ctx.SessionMeterpreterKill(ctx, mp.id)
	if err != nil {
		return err
	}
	_, err = mp.pw.Write([]byte("\r\n\r\n"))
	return err
}

// RunSingle is used to run single command.
func (mp *Meterpreter) RunSingle(ctx context.Context, cmd string) error {
	return mp.ctx.SessionMeterpreterRunSingle(ctx, mp.id, cmd)
}

// CompatibleModules is used to return a list of Post modules that compatible.
func (mp *Meterpreter) CompatibleModules(ctx context.Context) ([]string, error) {
	return mp.ctx.SessionCompatibleModules(ctx, mp.id)
}

// Close is used to close reader, it will not kill the meterpreter session.
func (mp *Meterpreter) Close() error {
	mp.destroy(true)
	return nil
}

func (mp *Meterpreter) closeNotWait() {
	mp.destroy(false)
}

func (mp *Meterpreter) destroy(wait bool) {
	mp.close()
	if wait {
		mp.wg.Wait()
	}
}

func (mp *Meterpreter) close() {
	mp.closeOnce.Do(func() {
		mp.cancel()
		_ = mp.pw.Close()
		_ = mp.pr.Close()
	})
}

// Kill is used to kill meterpreter session.
func (mp *Meterpreter) Kill() error {
	err := mp.ctx.SessionStop(mp.context, mp.id)
	if err != nil {
		return err
	}
	return mp.Close()
}
