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

// ModuleEncoders is used to returns a list of all loaded encoder modules in the
// framework instance. Note that the encoder/ prefix is not included in the path
// name of the return module.
func (msf *MSFRPC) ModuleEncoders(ctx context.Context) ([]string, error) {
	request := ModuleEncodersRequest{
		Method: MethodModuleEncoders,
		Token:  msf.GetToken(),
	}
	var result ModuleEncodersResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, &result.MSFError
	}
	return result.Modules, nil
}

// ModuleNops is used to returns a list of all loaded nop modules in the
// framework instance. Note that the nop/ prefix is not included in the path
// name of the return module.
func (msf *MSFRPC) ModuleNops(ctx context.Context) ([]string, error) {
	request := ModuleNopsRequest{
		Method: MethodModuleNops,
		Token:  msf.GetToken(),
	}
	var result ModuleNopsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, &result.MSFError
	}
	return result.Modules, nil
}

// ModuleEvasion is used to returns a list of all loaded evasion modules in the
// framework instance. Note that the evasion/ prefix is not included in the path
// name of the return module.
func (msf *MSFRPC) ModuleEvasion(ctx context.Context) ([]string, error) {
	request := ModuleEvasionRequest{
		Method: MethodModuleEvasion,
		Token:  msf.GetToken(),
	}
	var result ModuleEvasionResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, &result.MSFError
	}
	return result.Modules, nil
}

// ModuleInfo is used to get information and options about module.
func (msf *MSFRPC) ModuleInfo(ctx context.Context, typ, name string) (*ModuleInfoResult, error) {
	request := ModuleInfoRequest{
		Method: MethodModuleInfo,
		Token:  msf.GetToken(),
		Type:   typ,
		Name:   name,
	}
	var result ModuleInfoResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, &result.MSFError
	}
	return &result, nil
}
