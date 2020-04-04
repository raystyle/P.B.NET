package msfrpc

import (
	"context"
	"fmt"

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
		if result.ErrorMessage == "Unknown Session ID" {
			const format = "unknown session id: %d"
			result.ErrorMessage = fmt.Sprintf(format, id)
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}
