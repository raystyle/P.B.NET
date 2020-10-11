package msfrpc

import (
	"context"

	"github.com/pkg/errors"
)

// CoreModuleStats is used to return the number of modules loaded, broken down by type.
func (client *Client) CoreModuleStats(ctx context.Context) (*CoreModuleStatsResult, error) {
	request := CoreModuleStatsRequest{
		Method: MethodCoreModuleStats,
		Token:  client.GetToken(),
	}
	var result CoreModuleStatsResult
	err := client.send(ctx, &request, &result)
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

// CoreAddModulePath is used to add a new local file system directory (local to the server)
// as a module path. This can be used to dynamically load a separate module tree through
// the API. The path must be accessible to the user ID running the Metasploit service and
// contain a top-level directory for each module type (exploits, nop, encoder, payloads,
// auxiliary, post). Module paths will be immediately scanned for new modules and modules
// that loaded successfully will be immediately available. Note that this will not unload
// modules that were deleted from the file system since previously loaded (to remove all
// deleted modules, the core.reload_modules method should be used instead). This module may
// raise an error response if the specified path does not exist.
func (client *Client) CoreAddModulePath(ctx context.Context, path string) (*CoreAddModulePathResult, error) {
	request := CoreAddModulePathRequest{
		Method: MethodCoreAddModulePath,
		Token:  client.GetToken(),
		Path:   path,
	}
	var result CoreAddModulePathResult
	err := client.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "The path supplied is not a valid directory.":
			result.ErrorMessage = path + " is not a valid directory"
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return &result, nil
}

// CoreReloadModules is used to dump and reload all modules from all configured module
// paths. This is the only way to purge a previously loaded module that the caller
// would like to remove.
func (client *Client) CoreReloadModules(ctx context.Context) (*CoreReloadModulesResult, error) {
	request := CoreReloadModulesRequest{
		Method: MethodCoreReloadModules,
		Token:  client.GetToken(),
	}
	var result CoreReloadModulesResult
	err := client.send(ctx, &request, &result)
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

// CoreThreadList is used to return a list the status of all background threads along
// with an ID number that can be used to shut down the thread.
func (client *Client) CoreThreadList(ctx context.Context) (map[uint64]*CoreThreadInfo, error) {
	request := CoreThreadListRequest{
		Method: MethodCoreThreadList,
		Token:  client.GetToken(),
	}
	var (
		result   map[uint64]*CoreThreadInfo
		msfError MSFError
	)
	err := client.sendWithReplace(ctx, &request, &result, &msfError)
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

// CoreThreadKill is used to kill an errant background thread. The ThreadID should
// match what was returned by the core.thread_list method.
func (client *Client) CoreThreadKill(ctx context.Context, id uint64) error {
	request := CoreThreadKillRequest{
		Method: MethodCoreThreadKill,
		Token:  client.GetToken(),
		ID:     id,
	}
	var result CoreThreadKillResult
	err := client.send(ctx, &request, &result)
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

// CoreSetG is used to set a global data store value in the framework instance of the
// server. Examples of things that can be set include normal globals like LogLevel,
// but also the fallback for any modules launched from this point on. For example, the
// Proxies global option can be set, which would indicate that all modules launched
// from that point on should go through a specific chain of proxies, unless the Proxies
// option is specifically overridden for that module.
func (client *Client) CoreSetG(ctx context.Context, name, value string) error {
	request := CoreSetGRequest{
		Method: MethodCoreSetG,
		Token:  client.GetToken(),
		Name:   name,
		Value:  value,
	}
	var result CoreSetGResult
	err := client.send(ctx, &request, &result)
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

// CoreUnsetG is used to unset (delete) a previously configured global option.
func (client *Client) CoreUnsetG(ctx context.Context, name string) error {
	request := CoreUnsetGRequest{
		Method: MethodCoreUnsetG,
		Token:  client.GetToken(),
		Name:   name,
	}
	var result CoreUnsetGResult
	err := client.send(ctx, &request, &result)
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

// CoreGetG is used to get global setting by name, If the option is not set,
// then the value is empty.
func (client *Client) CoreGetG(ctx context.Context, name string) (string, error) {
	request := CoreGetGRequest{
		Method: MethodCoreGetG,
		Token:  client.GetToken(),
		Name:   name,
	}
	var (
		result   map[string]string
		msfError MSFError
	)
	err := client.sendWithReplace(ctx, &request, &result, &msfError)
	if err != nil {
		return "", err
	}
	if msfError.Err {
		if msfError.ErrorMessage == ErrInvalidToken {
			msfError.ErrorMessage = ErrInvalidTokenFriendly
		}
		return "", errors.WithStack(&msfError)
	}
	return result[name], nil
}

// CoreSave is used to save current global data store of the framework instance
// to server's disk. This configuration will be loaded by default the next time
// Metasploit is started by that user on that server.
func (client *Client) CoreSave(ctx context.Context) error {
	request := CoreSaveRequest{
		Method: MethodCoreSave,
		Token:  client.GetToken(),
	}
	var result CoreSaveResult
	err := client.send(ctx, &request, &result)
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

// CoreVersion is used to get basic version information about the running framework
// instance, the Ruby interpreter, and the RPC protocol version being used.
func (client *Client) CoreVersion(ctx context.Context) (*CoreVersionResult, error) {
	request := CoreVersionRequest{
		Method: MethodCoreVersion,
		Token:  client.GetToken(),
	}
	var result CoreVersionResult
	err := client.send(ctx, &request, &result)
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
