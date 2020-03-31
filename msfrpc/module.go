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

// ModuleAuxiliary is used to returns a list of all loaded auxiliary modules in the
// framework instance. Note that the auxiliary/ prefix is not included in the path
// name of the return module.
func (msf *MSFRPC) ModuleAuxiliary(ctx context.Context) ([]string, error) {
	request := ModuleAuxiliaryRequest{
		Method: MethodModuleAuxiliary,
		Token:  msf.GetToken(),
	}
	var result ModuleAuxiliaryResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, &result.MSFError
	}
	return result.Modules, nil
}

// ModulePost is used to returns a list of all loaded post modules in the framework
// instance. Note that the post/ prefix is not included in the path name of the
// return module.
func (msf *MSFRPC) ModulePost(ctx context.Context) ([]string, error) {
	request := ModulePostRequest{
		Method: MethodModulePost,
		Token:  msf.GetToken(),
	}
	var result ModulePostResult
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
