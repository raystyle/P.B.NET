package msfrpc

import (
	"context"
	"strconv"

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
