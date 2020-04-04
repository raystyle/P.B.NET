package msfrpc

import (
	"context"

	"github.com/pkg/errors"
)

// PluginLoad is used to load the specified plugin in the framework instance. The Options
// parameter can be used to specify initialization options to the plugin. The individual
// options are different for each plugin.
func (msf *MSFRPC) PluginLoad(ctx context.Context, name string, opts map[string]string) error {
	request := PluginLoadRequest{
		Method:  MethodPluginLoad,
		Token:   msf.GetToken(),
		Name:    name,
		Options: opts,
	}
	var result PluginLoadResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		return errors.WithStack(&result.MSFError)
	}
	if result.Result != "success" {
		const format = "failed to load plugin %s: %s"
		return errors.Errorf(format, name, result.Result)
	}
	return nil
}

// PluginUnload is used to unload a previously loaded plugin by name. The name is not
// always identical to the string used to load the plugin in the first place, so callers
// should check the output of plugin.loaded when there is any confusion.
func (msf *MSFRPC) PluginUnload(ctx context.Context, name string) error {
	request := PluginUnloadRequest{
		Method: MethodPluginUnload,
		Token:  msf.GetToken(),
		Name:   name,
	}
	var result PluginUnloadResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		return errors.WithStack(&result.MSFError)
	}
	if result.Result != "success" {
		const format = "failed to unload plugin %s: %s"
		return errors.Errorf(format, name, result.Result)
	}
	return nil
}

// PluginLoaded is used to enumerate all currently loaded plugins.
func (msf *MSFRPC) PluginLoaded(ctx context.Context) ([]string, error) {
	request := PluginLoadedRequest{
		Method: MethodPluginLoaded,
		Token:  msf.GetToken(),
	}
	var result PluginLoadedResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Plugins, nil
}
