package msfrpc

import (
	"context"

	"github.com/pkg/errors"
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
		return nil, errors.WithStack(&result.MSFError)
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
		return nil, errors.WithStack(&result.MSFError)
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
		return nil, errors.WithStack(&result.MSFError)
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
		return nil, errors.WithStack(&result.MSFError)
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
		return nil, errors.WithStack(&result.MSFError)
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
		return nil, errors.WithStack(&result.MSFError)
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
		return nil, errors.WithStack(&result.MSFError)
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
		return nil, errors.WithStack(&result.MSFError)
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
		return nil, errors.WithStack(&msfError)
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
		return nil, errors.WithStack(&result.MSFError)
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
		return nil, errors.WithStack(&result.MSFError)
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
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Sessions, nil
}

// ModuleCompatibleEvasionPayloads is used to returns a list of payloads that are
// compatible with the evasion module.
func (msf *MSFRPC) ModuleCompatibleEvasionPayloads(
	ctx context.Context,
	name string,
) ([]string, error) {
	request := ModuleCompatibleEvasionPayloadsRequest{
		Method: MethodModuleCompatibleEvasionPayloads,
		Token:  msf.GetToken(),
		Name:   name,
	}
	var result ModuleCompatibleEvasionPayloadsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Payloads, nil
}

// ModuleTargetCompatibleEvasionPayloads is used to returns the compatible target-specific
// payloads for an evasion module.
func (msf *MSFRPC) ModuleTargetCompatibleEvasionPayloads(
	ctx context.Context,
	name string,
	target uint64,
) ([]string, error) {
	request := ModuleTargetCompatibleEvasionPayloadsRequest{
		Method: MethodModuleTargetCompatibleEvasionPayloads,
		Token:  msf.GetToken(),
		Name:   name,
		Target: target,
	}
	var result ModuleTargetCompatibleEvasionPayloadsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Payloads, nil
}

// ModuleEncodeFormats is used to returns a list of encoding formats.
func (msf *MSFRPC) ModuleEncodeFormats(ctx context.Context) ([]string, error) {
	request := ModuleEncodeFormatsRequest{
		Method: MethodModuleEncodeFormats,
		Token:  msf.GetToken(),
	}
	var (
		result   []string
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

// ModuleExecutableFormats is used to returns a list of executable format names.
func (msf *MSFRPC) ModuleExecutableFormats(ctx context.Context) ([]string, error) {
	request := ModuleExecutableFormatsRequest{
		Method: MethodModuleExecutableFormats,
		Token:  msf.GetToken(),
	}
	var (
		result   []string
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

// ModuleTransformFormats is used to returns a list of transform format names.
func (msf *MSFRPC) ModuleTransformFormats(ctx context.Context) ([]string, error) {
	request := ModuleTransformFormatsRequest{
		Method: MethodModuleTransformFormats,
		Token:  msf.GetToken(),
	}
	var (
		result   []string
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

// ModuleEncryptionFormats is used to returns a list of encryption format names.
func (msf *MSFRPC) ModuleEncryptionFormats(ctx context.Context) ([]string, error) {
	request := ModuleEncryptionFormatsRequest{
		Method: MethodModuleEncryptionFormats,
		Token:  msf.GetToken(),
	}
	var (
		result   []string
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

// ModuleEncode is used to provide a way to encode an arbitrary payload (specified
// as Data) with a specific encoder and set of options.
func (msf *MSFRPC) ModuleEncode(
	ctx context.Context,
	data string,
	encoder string,
	opts *ModuleEncodeOptions,
) (string, error) {
	request := ModuleEncodeRequest{
		Method:  MethodModuleEncode,
		Token:   msf.GetToken(),
		Data:    data,
		Encoder: encoder,
		Options: opts.toMap(),
	}
	var result ModuleEncodeResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return "", err
	}
	if result.Err {
		return "", errors.WithStack(&result.MSFError)
	}
	return result.Encoded, nil
}

// ModuleExecute is used to provide a way to launch an exploit, run an auxiliary module,
// trigger a post module on a session, or generate a payload. The ModuleType should be
// one "exploit", "auxiliary", "post", and "payload". The ModuleName can either include
// module type prefix (exploit/) or not. The Datastore is the full set of datastore options
// that should be applied to the module before executing it.
//
// In the case of exploits, auxiliary, or post modules, the server response will return
// the Job ID of the running module
//
// In the case of payload modules, a number of additional options are parsed, including
// the datastore for the payload itself.
//
// parameter opts must be map[string]string or *ModuleExecuteOptions.
func (msf *MSFRPC) ModuleExecute(
	ctx context.Context,
	typ string,
	name string,
	opts interface{},
) (*ModuleExecuteResult, error) {
	request := ModuleExecuteRequest{
		Method: MethodModuleExecute,
		Token:  msf.GetToken(),
		Type:   typ,
		Name:   name,
	}
	switch typ {
	case "exploit", "auxiliary", "post":
		request.Options = opts.(map[string]interface{})
	case "payload": // generate payload
		request.Options = opts.(*ModuleExecuteOptions).toMap()
	default:
		return nil, errors.New("invalid module type: " + typ)
	}
	var result ModuleExecuteResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, errors.WithStack(&result.MSFError)
	}
	return &result, nil
}

// ModuleCheck is used to check exploit and auxiliary module.
func (msf *MSFRPC) ModuleCheck(
	ctx context.Context,
	typ string,
	name string,
	opts map[string]interface{},
) (*ModuleCheckResult, error) {
	request := ModuleCheckRequest{
		Method:  MethodModuleCheck,
		Token:   msf.GetToken(),
		Type:    typ,
		Name:    name,
		Options: opts,
	}
	var result ModuleCheckResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, errors.WithStack(&result.MSFError)
	}
	return &result, nil
}
