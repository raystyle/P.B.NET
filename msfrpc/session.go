package msfrpc

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
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
			result.ErrorMessage = "unknown session id: " + id
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
		case "Unknown Session ID " + id:
			result.ErrorMessage = "unknown session id: " + id
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
		case "Unknown Session ID " + id:
			result.ErrorMessage = "unknown session id: " + id
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return 0, errors.WithStack(&result.MSFError)
	}
	n, _ := strconv.ParseUint(result.WriteCount, 10, 64)
	return n, nil
}

// SessionUpgrade is used to attempt to spawn a new Meterpreter session through an
// existing Shell session. This requires that a multi/handler be running
// (windows/meterpreter/reverse_tcp) and that the host and port of this handler is
// provided to this method.
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
	list, err := msf.SessionList(ctx)
	if err != nil {
		return nil, err
	}
	info := list[id]
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
		case "Unknown Session ID " + id:
			result.ErrorMessage = "unknown session id: " + id
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
		case "Unknown Session ID " + id:
			result.ErrorMessage = "unknown session id: " + id
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
		case "Unknown Session ID " + id:
			result.ErrorMessage = "unknown session id: " + id
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}
