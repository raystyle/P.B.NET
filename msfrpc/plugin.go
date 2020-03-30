package msfrpc

import (
	"github.com/pkg/errors"
)

// PluginLoad is used to load the specified plugin in the framework instance. The Options
// parameter can be used to specify initialization options to the plugin. The individual
// options are different for each plugin.
func (msf *MSFRPC) PluginLoad(name string, opts map[string]string) error {
	request := PluginLoadRequest{
		Method:  MethodPluginLoad,
		Token:   msf.GetToken(),
		Name:    name,
		Options: opts,
	}
	var result PluginLoadResult
	err := msf.send(msf.ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		return &result.MSFError
	}
	if result.Result != "success" {
		const format = "failed to load plugin %s: %s"
		return errors.Errorf(format, name, result.Result)
	}
	return nil
}
