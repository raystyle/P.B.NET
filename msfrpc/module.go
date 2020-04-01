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

// ModuleInfo is used to returns a hash of detailed information about the specified
// module. The ModuleType should be one "exploit", "auxiliary", "post", "payload",
// "encoder", and "nop". The ModuleName can either include module type prefix
// (exploit/) or not.
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

// ModuleOptions is used to returns a hash of datastore options for the specified module.
// The ModuleType should be one "exploit", "auxiliary", "post", "payload", "encoder", and
// "nop". The ModuleName can either include module type prefix (exploit/) or not.
func (msf *MSFRPC) ModuleOptions(
	ctx context.Context,
	typ string,
	name string,
) (map[string]*ModuleSpecialOption, error) {
	request := ModuleOptionsRequest{
		Method: MethodModuleOptions,
		Token:  msf.GetToken(),
		Type:   typ,
		Name:   name,
	}
	var (
		result   map[string]*ModuleSpecialOption
		msfError MSFError
	)
	err := msf.sendWithReplace(ctx, &request, &result, &msfError)
	if err != nil {
		return nil, err
	}
	if msfError.Err {
		return nil, &msfError
	}
	return result, nil
}

// ModuleCompatiblePayloads is used to returns a list of payloads that are compatible
// with the exploit module name specified.
func (msf *MSFRPC) ModuleCompatiblePayloads(ctx context.Context, name string) ([]string, error) {
	request := ModuleCompatiblePayloadsRequest{
		Method: MethodModuleCompatiblePayloads,
		Token:  msf.GetToken(),
		Name:   name,
	}
	var result ModuleCompatiblePayloadsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, &result.MSFError
	}
	return result.Payloads, nil
}

// ModuleTargetCompatiblePayloads is similar to the module.compatible_payloads method
// in that it returns a list of matching payloads, however, it restricts those payloads
// to those that will work for a specific exploit target. For exploit modules that can
// attack multiple platforms and operating systems, this is the method used to obtain
// a list of available payloads after a target has been chosen.
func (msf *MSFRPC) ModuleTargetCompatiblePayloads(
	ctx context.Context,
	name string,
	target uint64,
) ([]string, error) {
	request := ModuleTargetCompatiblePayloadsRequest{
		Method: MethodModuleTargetCompatiblePayloads,
		Token:  msf.GetToken(),
		Name:   name,
		Target: target,
	}
	var result ModuleTargetCompatiblePayloadsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, &result.MSFError
	}
	return result.Payloads, nil
}

// ModuleCompatibleSessions is used to returns a list of payloads that are compatible
// with the post module name specified.
func (msf *MSFRPC) ModuleCompatibleSessions(ctx context.Context, name string) ([]string, error) {
	request := ModuleCompatibleSessionsRequest{
		Method: MethodModuleCompatibleSessions,
		Token:  msf.GetToken(),
		Name:   name,
	}
	var result ModuleCompatibleSessionsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, &result.MSFError
	}
	return result.Sessions, nil
}
