package msfrpc

import (
	"context"

	"github.com/pkg/errors"

	"project/internal/logger"
)

// AuthLogin is used to login metasploit RPC and get a temporary token. if use
// permanent token, dont need to call AuthLogin() but need AuthLogout().
func (msf *MSFRPC) AuthLogin() error {
	request := AuthLoginRequest{
		Method:   MethodAuthLogin,
		Username: msf.username,
		Password: msf.password,
	}
	var result AuthLoginResult
	err := msf.send(msf.ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		return errors.WithStack(&result.MSFError)
	}
	msf.SetToken(result.Token)
	msf.log(logger.Info, "login successfully")
	return nil
}

// AuthLogout is used to remove the specified token from the authentication token list.
// Note that this method can be used to disable any temporary token, not just the one
// used by the current user. The permanent token will not be removed.
func (msf *MSFRPC) AuthLogout(token string) error {
	request := AuthLogoutRequest{
		Method:      MethodAuthLogout,
		Token:       msf.GetToken(),
		LogoutToken: token,
	}
	var result AuthLogoutResult
	err := msf.send(msf.ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		if result.ErrorMessage == ErrInvalidToken {
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	msf.log(logger.Info, "logout:", token)
	return nil
}

// AuthTokenList is used to get token list.
func (msf *MSFRPC) AuthTokenList(ctx context.Context) ([]string, error) {
	request := AuthTokenListRequest{
		Method: MethodAuthTokenList,
		Token:  msf.GetToken(),
	}
	var result AuthTokenListResult
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
	return result.Tokens, nil
}

// AuthTokenGenerate is used to create a random 32-byte authentication token,
// add this token to the authenticated list, and return this token.
func (msf *MSFRPC) AuthTokenGenerate(ctx context.Context) (string, error) {
	request := AuthTokenGenerateRequest{
		Method: MethodAuthTokenGenerate,
		Token:  msf.GetToken(),
	}
	var result AuthTokenGenerateResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return "", err
	}
	if result.Err {
		if result.ErrorMessage == ErrInvalidToken {
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return "", errors.WithStack(&result.MSFError)
	}
	msf.log(logger.Info, "generate token")
	return result.Token, nil
}

// AuthTokenAdd is used to add an arbitrary string as a valid permanent authentication
// token. This token can be used for all future authentication purposes.
func (msf *MSFRPC) AuthTokenAdd(ctx context.Context, token string) error {
	request := AuthTokenAddRequest{
		Method:   MethodAuthTokenAdd,
		Token:    msf.GetToken(),
		NewToken: token,
	}
	var result AuthTokenAddResult
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
	msf.log(logger.Info, "add token")
	return nil
}

// AuthTokenRemove is used to delete a specified token. This will work for both
// temporary and permanent tokens, including those stored in the database backend.
func (msf *MSFRPC) AuthTokenRemove(ctx context.Context, token string) error {
	request := AuthTokenRemoveRequest{
		Method:           MethodAuthTokenRemove,
		Token:            msf.GetToken(),
		TokenToBeRemoved: token,
	}
	var result AuthTokenRemoveResult
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
	msf.log(logger.Info, "remove token:", token)
	return nil
}
