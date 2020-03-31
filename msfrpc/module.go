package msfrpc

import (
	"context"
)

// ModuleExploits is used to returns a list of all loaded exploit modules in the
// framework instance. Note that the exploit/ prefix is not included in the path
// name of the return module.
func (msf *MSFRPC) ModuleExploits(ctx context.Context) ([]string, error) {
	request := ModuleExploitsRequest{
		Method: MethodModuleExploits,
		Token:  msf.GetToken(),
	}
	var result ModuleExploitsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, &result.MSFError
	}
	return result.Modules, nil
}

// ModulePayloads is used to returns a list of all loaded payload modules in the
// framework instance. Note that the payload/ prefix is not included in the path
// name of the return module.
func (msf *MSFRPC) ModulePayloads(ctx context.Context) ([]string, error) {
	request := ModulePayloadsRequest{
		Method: MethodModulePayloads,
		Token:  msf.GetToken(),
	}
	var result ModulePayloadsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, &result.MSFError
	}
	return result.Modules, nil
}
