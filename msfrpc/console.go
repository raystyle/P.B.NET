package msfrpc

import (
	"context"

	"github.com/pkg/errors"
)

// ConsoleCreate is used to allocate a new console instance. The server will return a
// Console ID ("id") that is required to read, write, and otherwise interact with the
// new console. The "prompt" element in the return value indicates the current prompt
// for the console, which may include terminal sequences. Finally, the "busy" element
// of the return value determines whether the console is still processing the last
// command (in this case, it always be false). Note that while Console IDs are currently
// integers stored as strings, these may change to become alphanumeric strings in the
// future. Callers should treat Console IDs as unique strings, not integers, wherever
// possible.
func (msf *MSFRPC) ConsoleCreate(ctx context.Context) (*ConsoleCreateResult, error) {
	request := ConsoleCreateRequest{
		Method: MethodConsoleCreate,
		Token:  msf.GetToken(),
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
