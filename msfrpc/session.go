package msfrpc

import (
	"context"

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
