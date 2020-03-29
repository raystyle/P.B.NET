package msfrpc

import (
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
func (msf *MSFRPC) ConsoleCreate() (*ConsoleCreateResult, error) {
	request := ConsoleCreateRequest{
		Method: MethodConsoleCreate,
		Token:  msf.GetToken(),
	}
	var result ConsoleCreateResult
	err := msf.send(msf.ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, &result.MSFError
	}
	return &result, nil
}

// ConsoleDestroy is used to destroy a running console instance by Console ID. Consoles
// should always be destroyed after the caller is finished to prevent resource leaks on
// the server side. If an invalid Console ID is specified.
func (msf *MSFRPC) ConsoleDestroy(id string) error {
	request := ConsoleDestroyRequest{
		Method: MethodConsoleDestroy,
		Token:  msf.GetToken(),
		ID:     id,
	}
	var result ConsoleDestroyResult
	err := msf.send(msf.ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		return &result.MSFError
	}
	if result.Result != "success" {
		return errors.New("invalid console id: " + id)
	}
	return nil
}

// ConsoleList is used to return a hash of all existing Console IDs, their status,
// and their prompts.
func (msf *MSFRPC) ConsoleList() ([]*ConsoleInfo, error) {
	request := ConsoleListRequest{
		Method: MethodConsoleList,
		Token:  msf.GetToken(),
	}
	var result ConsoleListResult
	err := msf.send(msf.ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, &result.MSFError
	}
	return result.Consoles, nil
}
