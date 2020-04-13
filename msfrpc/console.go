package msfrpc

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/xpanic"
)

// ConsoleList is used to return a hash of all existing Console IDs, their status,
// and their prompts.
func (msf *MSFRPC) ConsoleList(ctx context.Context) ([]*ConsoleInfo, error) {
	request := ConsoleListRequest{
		Method: MethodConsoleList,
		Token:  msf.GetToken(),
	}
	var result ConsoleListResult
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
	return result.Consoles, nil
}

// ConsoleCreate is used to allocate a new console instance. The server will return a
// Console ID ("id") that is required to read, write, and otherwise interact with the
// new console. The "prompt" element in the return value indicates the current prompt
// for the console, which may include terminal sequences. Finally, the "busy" element
// of the return value determines whether the console is still processing the last
// command (in this case, it always be false). Note that while Console IDs are currently
// integers stored as strings, these may change to become alphanumeric strings in the
// future. Callers should treat Console IDs as unique strings, not integers, wherever
// possible.
func (msf *MSFRPC) ConsoleCreate(ctx context.Context, workspace string) (*ConsoleCreateResult, error) {
	opts := make(map[string]string, 1)
	if workspace == "" {
		opts["workspace"] = defaultWorkspace
	} else {
		opts["workspace"] = workspace
	}
	request := ConsoleCreateRequest{
		Method:  MethodConsoleCreate,
		Token:   msf.GetToken(),
		Options: opts,
	}
	var result ConsoleCreateResult
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
	msf.log(logger.Debug, "create console:", result.ID)
	return &result, nil
}

// ConsoleDestroy is used to destroy a running console instance by Console ID. Consoles
// should always be destroyed after the caller is finished to prevent resource leaks on
// the server side. If an invalid Console ID is specified.
func (msf *MSFRPC) ConsoleDestroy(ctx context.Context, id string) error {
	request := ConsoleDestroyRequest{
		Method: MethodConsoleDestroy,
		Token:  msf.GetToken(),
		ID:     id,
	}
	var result ConsoleDestroyResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		if result.ErrorMessage == ErrInvalidToken {
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	if result.Result != "success" {
		return errors.New("invalid console id: " + id)
	}
	msf.log(logger.Debug, "destroy console:", id)
	return nil
}

// ConsoleRead is used to return any output currently buffered by the console that has
// not already been read. The data is returned in the raw form printed by the actual
// console. Note that a newly allocated console will have the initial banner available
// to read.
func (msf *MSFRPC) ConsoleRead(ctx context.Context, id string) (*ConsoleReadResult, error) {
	request := ConsoleReadRequest{
		Method: MethodConsoleRead,
		Token:  msf.GetToken(),
		ID:     id,
	}
	var result ConsoleReadResult
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
	if result.Result != "" {
		const format = "failed to read from console %s: %s"
		return nil, errors.Errorf(format, id, result.Result)
	}
	return &result, nil
}

// ConsoleWrite is used to send data to a specific console, just as if it had been typed
// by a normal user. This means that most commands will need a newline included at the
// end for the console to process them properly.
func (msf *MSFRPC) ConsoleWrite(ctx context.Context, id, data string) (uint64, error) {
	if len(data) == 0 {
		return 0, nil
	}
	request := ConsoleWriteRequest{
		Method: MethodConsoleWrite,
		Token:  msf.GetToken(),
		ID:     id,
		Data:   data,
	}
	var result ConsoleWriteResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return 0, err
	}
	if result.Err {
		if result.ErrorMessage == ErrInvalidToken {
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return 0, errors.WithStack(&result.MSFError)
	}
	if result.Result != "" {
		const format = "failed to write to console %s: %s"
		return 0, errors.Errorf(format, id, result.Result)
	}
	return result.Wrote, nil
}

// ConsoleSessionDetach is used to background an interactive session in the Metasploit
// Framework Console. This method can be used to return to the main Metasploit prompt
// after entering an interactive session through a sessions –i console command or through
// an exploit.
func (msf *MSFRPC) ConsoleSessionDetach(ctx context.Context, id string) error {
	request := ConsoleSessionDetachRequest{
		Method: MethodConsoleSessionDetach,
		Token:  msf.GetToken(),
		ID:     id,
	}
	var result ConsoleSessionDetachResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		if result.ErrorMessage == ErrInvalidToken {
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	if result.Result != "success" {
		const format = "failed to detach session about console %s: %s"
		return errors.Errorf(format, id, result.Result)
	}
	return nil
}

// ConsoleSessionKill is used to abort an interactive session in the Metasploit Framework
// Console. This method should only be used after a sessions –i command has been written
// or an exploit was called through the Console API. In most cases, the session API methods
// are a better way to session termination, while the console.session_detach method is a
// better way to drop back to the main Metasploit console.
func (msf *MSFRPC) ConsoleSessionKill(ctx context.Context, id string) error {
	request := ConsoleSessionKillRequest{
		Method: MethodConsoleSessionKill,
		Token:  msf.GetToken(),
		ID:     id,
	}
	var result ConsoleSessionKillResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		if result.ErrorMessage == ErrInvalidToken {
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	if result.Result != "success" {
		const format = "failed to kill session about console %s: %s"
		return errors.Errorf(format, id, result.Result)
	}
	return nil
}

const minReadInterval = 50 * time.Millisecond

// Console is used to provide a more gracefully io. It implemented io.ReadWriteCloser.
type Console struct {
	ctx *MSFRPC

	id       string
	interval time.Duration

	logSrc   string
	pr       *io.PipeReader
	pw       *io.PipeWriter
	writeMu  sync.Mutex
	token    chan struct{}
	closed   bool
	closedMu sync.Mutex

	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewConsole is used to create a console, it will create a new console(msfrpc).
func (msf *MSFRPC) NewConsole(
	ctx context.Context,
	workspace string,
	interval time.Duration,
) (*Console, error) {
	result, err := msf.ConsoleCreate(ctx, workspace)
	if err != nil {
		return nil, err
	}
	return msf.NewConsoleWithID(result.ID, interval), nil
}

// NewConsoleWithID is used to create a graceful IO stream with console id.
// If appear some errors about network, you can use it to attach an exist console.
func (msf *MSFRPC) NewConsoleWithID(id string, interval time.Duration) *Console {
	if interval < minReadInterval {
		interval = minReadInterval
	}
	console := Console{
		ctx:      msf,
		id:       id,
		interval: interval,
		logSrc:   "msfrpc-console-" + id,
		token:    make(chan struct{}),
	}
	console.pr, console.pw = io.Pipe()
	console.context, console.cancel = context.WithCancel(context.Background())
	console.wg.Add(2)
	go console.reader()
	go console.writeLimiter()
	return &console
}

func (console *Console) log(lv logger.Level, log ...interface{}) {
	console.ctx.logger.Println(lv, console.logSrc, log...)
}

// reader is used to call MSFRPC.ConsoleRead() high frequency and write the output
// to a pipe and wait user call Read().
func (console *Console) reader() {
	defer func() {
		if r := recover(); r != nil {
			console.log(logger.Fatal, xpanic.Print(r, "Console.reader"))
			// restart reader
			time.Sleep(time.Second)
			go console.reader()
		} else {
			console.wg.Done()
		}
	}()
	// don't use ticker otherwise read write will appear confusion.
	timer := time.NewTimer(console.interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			if !console.read() {
				return
			}
		case <-console.context.Done():
			return
		}
		timer.Reset(console.interval)
	}
}

func (console *Console) read() bool {
	console.writeMu.Lock()
	defer console.writeMu.Unlock()

	var (
		result  *ConsoleReadResult
		err     error
		dataLen int
		busy    bool
		timer   *time.Timer
	)
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		result, err = console.ctx.ConsoleRead(console.context, console.id)
		if err != nil {
			return false
		}
		dataLen = len(result.Data)
		if result.Busy {
			if dataLen == 0 {
				// wait some time to read again when block
				// like input "use exploit/multi/handler"
				if timer == nil {
					timer = time.NewTimer(console.interval)
				} else {
					timer.Reset(console.interval)
				}
				select {
				case <-timer.C:
				case <-console.context.Done():
					return false
				}
				busy = true
				continue
			}
			// write output
			_, err = console.pw.Write([]byte(result.Data))
			if err != nil {
				return false
			}
			busy = true
			continue
		}
		// check busy is changed idle.
		if busy {
			if dataLen != 0 {
				_, err = console.pw.Write([]byte(result.Data))
				if err != nil {
					return false
				}
			}
			_, err = console.pw.Write([]byte(result.Prompt))
			return err == nil
		}
		// idle state
		if dataLen == 0 {
			return true
		}
		// write output
		_, err = console.pw.Write([]byte(result.Data))
		if err != nil {
			return false
		}
		_, err = console.pw.Write([]byte(result.Prompt))
		return err == nil
	}
}

func (console *Console) writeLimiter() {
	defer func() {
		if r := recover(); r != nil {
			console.log(logger.Fatal, xpanic.Print(r, "Console.writeLimiter"))
			// restart limiter
			time.Sleep(time.Second)
			go console.writeLimiter()
		} else {
			console.wg.Done()
		}
	}()
	// don't use ticker otherwise read write will appear confusion.
	interval := 2 * console.interval
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			select {
			case console.token <- struct{}{}:
			case <-console.context.Done():
				return
			}
		case <-console.context.Done():
			return
		}
		timer.Reset(interval)
	}
}

func (console *Console) Read(b []byte) (int, error) {
	return console.pr.Read(b)
}

func (console *Console) Write(b []byte) (int, error) {
	// block before first call read()
	select {
	case <-console.token:
	case <-console.context.Done():
		return 0, console.context.Err()
	}
	console.writeMu.Lock()
	defer console.writeMu.Unlock()
	// if read() in busy and return, this lock will
	// release at once, write will appear confusion.
	select {
	case <-console.token:
	case <-console.context.Done():
		return 0, console.context.Err()
	}
	n, err := console.ctx.ConsoleWrite(console.context, console.id, string(b))
	if err != nil {
		return int(n), err
	}
	// write to input data to output pipe
	return console.pw.Write(b)
}

// Detach is used to detach current console.
func (console *Console) Detach(ctx context.Context) error {
	return console.ctx.ConsoleSessionDetach(ctx, console.id)
}

// Interrupt is used to send interrupt to current console.
func (console *Console) Interrupt(ctx context.Context) error {
	return console.ctx.ConsoleSessionKill(ctx, console.id)
}

// Close is used to destroy console.
func (console *Console) Close() error {
	console.closedMu.Lock()
	defer console.closedMu.Unlock()
	if console.closed {
		return nil
	}
	err := console.ctx.ConsoleDestroy(console.context, console.id)
	if err != nil {
		return err
	}
	_ = console.pw.Close()
	_ = console.pr.Close()
	console.cancel()
	console.wg.Wait()
	console.closed = true
	return nil
}
