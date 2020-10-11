package msfrpc

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
)

// ModuleExploits is used to returns a list of all loaded exploit modules in the
// framework instance. Note that the exploit/ prefix is not included in the path
// name of the return module.
func (client *Client) ModuleExploits(ctx context.Context) ([]string, error) {
	request := ModuleExploitsRequest{
		Method: MethodModuleExploits,
		Token:  client.GetToken(),
	}
	var result ModuleExploitsResult
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
	return result.Modules, nil
}

// ModuleAuxiliary is used to returns a list of all loaded auxiliary modules in the
// framework instance. Note that the auxiliary/ prefix is not included in the path
// name of the return module.
func (client *Client) ModuleAuxiliary(ctx context.Context) ([]string, error) {
	request := ModuleAuxiliaryRequest{
		Method: MethodModuleAuxiliary,
		Token:  client.GetToken(),
	}
	var result ModuleAuxiliaryResult
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
	return result.Modules, nil
}

// ModulePost is used to returns a list of all loaded post modules in the framework
// instance. Note that the post/ prefix is not included in the path name of the
// return module.
func (client *Client) ModulePost(ctx context.Context) ([]string, error) {
	request := ModulePostRequest{
		Method: MethodModulePost,
		Token:  client.GetToken(),
	}
	var result ModulePostResult
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
	return result.Modules, nil
}

// ModulePayloads is used to returns a list of all loaded payload modules in the
// framework instance. Note that the payload/ prefix is not included in the path
// name of the return module.
func (client *Client) ModulePayloads(ctx context.Context) ([]string, error) {
	request := ModulePayloadsRequest{
		Method: MethodModulePayloads,
		Token:  client.GetToken(),
	}
	var result ModulePayloadsResult
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
	return result.Modules, nil
}

// ModuleEncoders is used to returns a list of all loaded encoder modules in the
// framework instance. Note that the encoder/ prefix is not included in the path
// name of the return module.
func (client *Client) ModuleEncoders(ctx context.Context) ([]string, error) {
	request := ModuleEncodersRequest{
		Method: MethodModuleEncoders,
		Token:  client.GetToken(),
	}
	var result ModuleEncodersResult
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
	return result.Modules, nil
}

// ModuleNops is used to returns a list of all loaded nop modules in the
// framework instance. Note that the nop/ prefix is not included in the path
// name of the return module.
func (client *Client) ModuleNops(ctx context.Context) ([]string, error) {
	request := ModuleNopsRequest{
		Method: MethodModuleNops,
		Token:  client.GetToken(),
	}
	var result ModuleNopsResult
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
	return result.Modules, nil
}

// ModuleEvasion is used to returns a list of all loaded evasion modules in the
// framework instance. Note that the evasion/ prefix is not included in the path
// name of the return module.
func (client *Client) ModuleEvasion(ctx context.Context) ([]string, error) {
	request := ModuleEvasionRequest{
		Method: MethodModuleEvasion,
		Token:  client.GetToken(),
	}
	var result ModuleEvasionResult
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
	return result.Modules, nil
}

// ModuleInfo is used to returns a hash of detailed information about the specified
// module. The ModuleType should be one "exploit", "auxiliary", "post", "payload",
// "encoder", and "nop". The ModuleName can either include module type prefix
// (exploit/) or not.
func (client *Client) ModuleInfo(ctx context.Context, typ, name string) (*ModuleInfoResult, error) {
	request := ModuleInfoRequest{
		Method: MethodModuleInfo,
		Token:  client.GetToken(),
		Type:   typ,
		Name:   name,
	}
	var result ModuleInfoResult
	err := client.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidModule:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidModuleFormat, typ, name)
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return &result, nil
}

// ModuleOptions is used to returns a hash of datastore options for the specified module.
// The ModuleType should be one "exploit", "auxiliary", "post", "payload", "encoder", and
// "nop". The ModuleName can either include module type prefix (exploit/) or not.
func (client *Client) ModuleOptions(ctx context.Context, typ, name string) (map[string]*ModuleSpecialOption, error) {
	request := ModuleOptionsRequest{
		Method: MethodModuleOptions,
		Token:  client.GetToken(),
		Type:   typ,
		Name:   name,
	}
	var (
		result   map[string]*ModuleSpecialOption
		msfError MSFError
	)
	err := client.sendWithReplace(ctx, &request, &result, &msfError)
	if err != nil {
		return nil, err
	}
	if msfError.Err {
		switch msfError.ErrorMessage {
		case ErrInvalidModule:
			msfError.ErrorMessage = fmt.Sprintf(ErrInvalidModuleFormat, typ, name)
		case ErrInvalidToken:
			msfError.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&msfError)
	}
	return result, nil
}

// ModuleCompatiblePayloads is used to returns a list of payloads that are compatible
// with the exploit module name specified.
func (client *Client) ModuleCompatiblePayloads(ctx context.Context, name string) ([]string, error) {
	request := ModuleCompatiblePayloadsRequest{
		Method: MethodModuleCompatiblePayloads,
		Token:  client.GetToken(),
		Name:   name,
	}
	var result ModuleCompatiblePayloadsResult
	err := client.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidModule:
			result.ErrorMessage = "invalid module: exploit/" + name
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Payloads, nil
}

// ModuleTargetCompatiblePayloads is similar to the module.compatible_payloads method
// in that it returns a list of matching payloads, however, it restricts those payloads
// to those that will work for a specific exploit target. For exploit modules that can
// attack multiple platforms and operating systems, this is the method used to obtain
// a list of available payloads after a target has been chosen.
func (client *Client) ModuleTargetCompatiblePayloads(
	ctx context.Context,
	name string,
	target uint64,
) ([]string, error) {
	request := ModuleTargetCompatiblePayloadsRequest{
		Method: MethodModuleTargetCompatiblePayloads,
		Token:  client.GetToken(),
		Name:   name,
		Target: target,
	}
	var result ModuleTargetCompatiblePayloadsResult
	err := client.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidModule:
			result.ErrorMessage = "invalid module: exploit/" + name
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Payloads, nil
}

// ModuleCompatibleSessions is used to returns a list of payloads that are compatible
// with the post module name specified.
func (client *Client) ModuleCompatibleSessions(ctx context.Context, name string) ([]string, error) {
	request := ModuleCompatibleSessionsRequest{
		Method: MethodModuleCompatibleSessions,
		Token:  client.GetToken(),
		Name:   name,
	}
	var result ModuleCompatibleSessionsResult
	err := client.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidModule:
			result.ErrorMessage = "invalid module: post/" + name
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Sessions, nil
}

// ModuleCompatibleEvasionPayloads is used to returns a list of payloads that are
// compatible with the evasion module.
func (client *Client) ModuleCompatibleEvasionPayloads(
	ctx context.Context,
	name string,
) ([]string, error) {
	request := ModuleCompatibleEvasionPayloadsRequest{
		Method: MethodModuleCompatibleEvasionPayloads,
		Token:  client.GetToken(),
		Name:   name,
	}
	var result ModuleCompatibleEvasionPayloadsResult
	err := client.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidModule:
			result.ErrorMessage = "invalid module: evasion/" + name
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Payloads, nil
}

// ModuleTargetCompatibleEvasionPayloads is used to returns the compatible target-specific
// payloads for an evasion module.
func (client *Client) ModuleTargetCompatibleEvasionPayloads(
	ctx context.Context,
	name string,
	target uint64,
) ([]string, error) {
	request := ModuleTargetCompatibleEvasionPayloadsRequest{
		Method: MethodModuleTargetCompatibleEvasionPayloads,
		Token:  client.GetToken(),
		Name:   name,
		Target: target,
	}
	var result ModuleTargetCompatibleEvasionPayloadsResult
	err := client.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidModule:
			result.ErrorMessage = "invalid module: evasion/" + name
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Payloads, nil
}

// ModuleEncodeFormats is used to returns a list of encoding formats.
func (client *Client) ModuleEncodeFormats(ctx context.Context) ([]string, error) {
	request := ModuleEncodeFormatsRequest{
		Method: MethodModuleEncodeFormats,
		Token:  client.GetToken(),
	}
	var (
		result   []string
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

// ModuleExecutableFormats is used to returns a list of executable format names.
func (client *Client) ModuleExecutableFormats(ctx context.Context) ([]string, error) {
	request := ModuleExecutableFormatsRequest{
		Method: MethodModuleExecutableFormats,
		Token:  client.GetToken(),
	}
	var (
		result   []string
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

// ModuleTransformFormats is used to returns a list of transform format names.
func (client *Client) ModuleTransformFormats(ctx context.Context) ([]string, error) {
	request := ModuleTransformFormatsRequest{
		Method: MethodModuleTransformFormats,
		Token:  client.GetToken(),
	}
	var (
		result   []string
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

// ModuleEncryptionFormats is used to returns a list of encryption format names.
func (client *Client) ModuleEncryptionFormats(ctx context.Context) ([]string, error) {
	request := ModuleEncryptionFormatsRequest{
		Method: MethodModuleEncryptionFormats,
		Token:  client.GetToken(),
	}
	var (
		result   []string
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

// ModulePlatforms is used to returns a list of platform names.
func (client *Client) ModulePlatforms(ctx context.Context) ([]string, error) {
	request := ModulePlatformsRequest{
		Method: MethodModulePlatforms,
		Token:  client.GetToken(),
	}
	var (
		result   []string
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

// ModuleArchitectures is used to returns a list of architecture names..
func (client *Client) ModuleArchitectures(ctx context.Context) ([]string, error) {
	request := ModuleArchitecturesRequest{
		Method: MethodModuleArchitectures,
		Token:  client.GetToken(),
	}
	var (
		result   []string
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

// ModuleEncode is used to provide a way to encode an arbitrary payload (specified
// as Data) with a specific encoder and set of options.
func (client *Client) ModuleEncode(
	ctx context.Context,
	data string,
	encoder string,
	opts *ModuleEncodeOptions,
) (string, error) {
	if len(data) == 0 {
		return "", errors.New("no data")
	}
	request := ModuleEncodeRequest{
		Method:  MethodModuleEncode,
		Token:   client.GetToken(),
		Data:    data,
		Encoder: encoder,
		Options: opts.toMap(),
	}
	var result ModuleEncodeResult
	err := client.send(ctx, &request, &result)
	if err != nil {
		return "", err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "Invalid Format: " + opts.Format:
			result.ErrorMessage = "invalid format: " + opts.Format
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
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
func (client *Client) ModuleExecute(
	ctx context.Context,
	typ string,
	name string,
	opts interface{},
) (*ModuleExecuteResult, error) {
	request := ModuleExecuteRequest{
		Method: MethodModuleExecute,
		Token:  client.GetToken(),
		Type:   typ,
		Name:   name,
	}
	switch typ {
	case "exploit", "auxiliary", "post", "evasion":
		request.Options = opts.(map[string]interface{})
	case "payload": // generate payload
		request.Options = opts.(*ModuleExecuteOptions).toMap()
	default:
		return nil, errors.New("invalid module type: " + typ)
	}
	var result ModuleExecuteResult
	err := client.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidModule:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidModuleFormat, typ, name)
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return &result, nil
}

// ModuleCheck is used to check exploit and auxiliary module.
func (client *Client) ModuleCheck(
	ctx context.Context,
	typ string,
	name string,
	opts map[string]interface{},
) (*ModuleCheckResult, error) {
	switch typ {
	case "exploit", "auxiliary":
	default:
		return nil, errors.New("invalid module type: " + typ)
	}
	request := ModuleCheckRequest{
		Method:  MethodModuleCheck,
		Token:   client.GetToken(),
		Type:    typ,
		Name:    name,
		Options: opts,
	}
	var result ModuleCheckResult
	err := client.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidModule:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidModuleFormat, typ, name)
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return &result, nil
}

// ModuleRunningStats is used to returns the currently running module stats in each state.
func (client *Client) ModuleRunningStats(ctx context.Context) (*ModuleRunningStatsResult, error) {
	request := ModuleRunningStatsRequest{
		Method: MethodModuleRunningStats,
		Token:  client.GetToken(),
	}
	var result ModuleRunningStatsResult
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
