package msfrpc

import (
	"context"

	"github.com/pkg/errors"
)

// DBConnect is used to connect database.
func (msf *MSFRPC) DBConnect(ctx context.Context, opts *DBConnectOptions) error {
	request := DBConnectRequest{
		Method:  MethodDBConnect,
		Token:   msf.GetToken(),
		Options: opts.toMap(),
	}
	var result DBConnectResult
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
		return errors.New("failed to connect database")
	}
	return nil
}

// DBDisconnect is used to disconnect database.
func (msf *MSFRPC) DBDisconnect(ctx context.Context) error {
	request := DBDisconnectRequest{
		Method: MethodDBDisconnect,
		Token:  msf.GetToken(),
	}
	var result DBDisconnectResult
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
	return nil
}

// DBStatus is used to get the database status.
func (msf *MSFRPC) DBStatus(ctx context.Context) (*DBStatusResult, error) {
	request := DBStatusRequest{
		Method: MethodDBStatus,
		Token:  msf.GetToken(),
	}
	var result DBStatusResult
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

// DBHosts is used to get all hosts information in the database.
func (msf *MSFRPC) DBHosts(ctx context.Context, workspace string) ([]*DBHost, error) {
	if workspace == "" {
		workspace = "default"
	}
	request := DBHostsRequest{
		Method: MethodDBHosts,
		Token:  msf.GetToken(),
		Options: map[string]interface{}{
			"workspace": workspace,
		},
	}
	var result DBHostsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "Invalid workspace":
			result.ErrorMessage = "invalid workspace: " + workspace
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Hosts, nil
}
