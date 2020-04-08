package msfrpc

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"project/internal/xreflect"
)

const defaultWorkspace = "default"

// DBConnect is used to connect database.
func (msf *MSFRPC) DBConnect(ctx context.Context, opts *DBConnectOptions) error {
	request := DBConnectRequest{
		Method:  MethodDBConnect,
		Token:   msf.GetToken(),
		Options: opts.toMap(),
	}
	var result DBConnectResult
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
	if result.Result != "success" {
		return errors.New("failed to connect database")
	}
	return nil
}

// DBDisconnect is used to disconnect database.
func (msf *MSFRPC) DBDisconnect(ctx context.Context) error {
	request := DBDisconnectRequest{
		Method: MethodDBDisconnect,
		Token:  msf.GetToken(),
	}
	var result DBDisconnectResult
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
	return nil
}

// DBStatus is used to get the database status.
func (msf *MSFRPC) DBStatus(ctx context.Context) (*DBStatusResult, error) {
	request := DBStatusRequest{
		Method: MethodDBStatus,
		Token:  msf.GetToken(),
	}
	var result DBStatusResult
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
	return &result, nil
}

// DBReportHost is used to add host to database.
func (msf *MSFRPC) DBReportHost(ctx context.Context, host *DBReportHost) error {
	request := DBReportHostRequest{
		Method: MethodDBReportHost,
		Token:  msf.GetToken(),
		Host:   xreflect.StructureToMap(host, structTag),
	}
	var result DBReportHostResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "Invalid workspace":
			result.ErrorMessage = "invalid workspace: " + host.Workspace
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// DBHosts is used to get all hosts information in the database.
func (msf *MSFRPC) DBHosts(ctx context.Context, workspace string) ([]*DBHost, error) {
	if workspace == "" {
		workspace = defaultWorkspace
	}
	request := DBHostsRequest{
		Method: MethodDBHosts,
		Token:  msf.GetToken(),
		Options: map[string]interface{}{
			"workspace": workspace,
		},
	}
	var result DBHostsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "Invalid workspace":
			result.ErrorMessage = "invalid workspace: " + workspace
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Hosts, nil
}

// DBGetHost is used to get host with workspace or address.
func (msf *MSFRPC) DBGetHost(ctx context.Context, workspace, address string) (*DBHost, error) {
	if workspace == "" {
		workspace = defaultWorkspace
	}
	opts := map[string]interface{}{
		"workspace": workspace,
	}
	if address != "" {
		opts["address"] = address
	}
	request := DBGetHostRequest{
		Method:  MethodDBGetHost,
		Token:   msf.GetToken(),
		Options: opts,
	}
	var result DBGetHostResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "Invalid workspace":
			result.ErrorMessage = "invalid workspace: " + workspace
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	if len(result.Host) == 0 {
		return nil, errors.Errorf("host: %s doesn't exist", address)
	}
	return result.Host[0], nil
}

// DBDelHost is used to delete host by filters, it will return deleted host.
func (msf *MSFRPC) DBDelHost(ctx context.Context, workspace, address string) ([]string, error) {
	if workspace == "" {
		workspace = defaultWorkspace
	}
	opts := map[string]interface{}{
		"workspace": workspace,
	}
	if address != "" {
		opts["address"] = address
	}
	request := DBDelHostRequest{
		Method:  MethodDBDelHost,
		Token:   msf.GetToken(),
		Options: opts,
	}
	var result DBDelHostResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "Invalid workspace":
			result.ErrorMessage = "invalid workspace: " + workspace
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Deleted, nil
}

// DBReportService is used to add service to database.
func (msf *MSFRPC) DBReportService(ctx context.Context, service *DBReportService) error {
	request := DBReportServiceRequest{
		Method:  MethodDBReportService,
		Token:   msf.GetToken(),
		Service: xreflect.StructureToMap(service, structTag),
	}
	var result DBReportServiceResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "Invalid workspace":
			result.ErrorMessage = "invalid workspace: " + service.Workspace
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// DBServices is used to get services by filter options.
func (msf *MSFRPC) DBServices(ctx context.Context, opts *DBServicesOptions) ([]*DBService, error) {
	cOpts := *opts
	if cOpts.Workspace == "" {
		cOpts.Workspace = defaultWorkspace
	}
	request := DBServicesRequest{
		Method:  MethodDBServices,
		Token:   msf.GetToken(),
		Options: xreflect.StructureToMap(&cOpts, structTag),
	}
	var result DBServicesResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "Invalid workspace":
			result.ErrorMessage = "invalid workspace: " + opts.Workspace
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Services, nil
}

// DBGetService is used to get services by filter.
func (msf *MSFRPC) DBGetService(
	ctx context.Context,
	opts *DBGetServiceOptions,
) ([]*DBService, error) {
	cOpts := *opts
	if cOpts.Workspace == "" {
		cOpts.Workspace = defaultWorkspace
	}
	request := DBGetServiceRequest{
		Method:  MethodDBGetService,
		Token:   msf.GetToken(),
		Options: xreflect.StructureToMap(&cOpts, structTag),
	}
	var result DBGetServiceResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "Invalid workspace":
			result.ErrorMessage = "invalid workspace: " + opts.Workspace
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Service, nil
}

// DBDelService is used to delete service by filter.
func (msf *MSFRPC) DBDelService(
	ctx context.Context,
	opts *DBDelServiceOptions,
) ([]*DBDelService, error) {
	cOpts := *opts
	if cOpts.Workspace == "" {
		cOpts.Workspace = defaultWorkspace
	}
	request := DBDelServiceRequest{
		Method:  MethodDBDelService,
		Token:   msf.GetToken(),
		Options: xreflect.StructureToMap(&cOpts, structTag),
	}
	var result DBDelServiceResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "Invalid workspace":
			result.ErrorMessage = "invalid workspace: " + opts.Workspace
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Deleted, nil
}

// DBWorkspaces is used to get information about workspaces.
func (msf *MSFRPC) DBWorkspaces(ctx context.Context) ([]*DBWorkspace, error) {
	request := DBWorkspacesRequest{
		Method: MethodDBWorkspaces,
		Token:  msf.GetToken(),
	}
	var result DBWorkspacesResult
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
	return result.Workspaces, nil
}

// DBGetWorkspace is used to get workspace information by name.
func (msf *MSFRPC) DBGetWorkspace(ctx context.Context, name string) (*DBWorkspace, error) {
	request := DBGetWorkspaceRequest{
		Method: MethodDBGetWorkspace,
		Token:  msf.GetToken(),
		Name:   name,
	}
	var result DBGetWorkspaceResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, name)
		case ErrDBNotLoaded:
			result.ErrorMessage = ErrDBNotLoadedFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Workspace[0], nil
}

// DBAddWorkspace is used to add workspace.
func (msf *MSFRPC) DBAddWorkspace(ctx context.Context, name string) error {
	request := DBAddWorkspaceRequest{
		Method: MethodDBAddWorkspace,
		Token:  msf.GetToken(),
		Name:   name,
	}
	var result DBAddWorkspaceResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// DBDelWorkspace is used to delete workspace by name.
func (msf *MSFRPC) DBDelWorkspace(ctx context.Context, name string) error {
	request := DBDelWorkspaceRequest{
		Method: MethodDBDelWorkspace,
		Token:  msf.GetToken(),
		Name:   name,
	}
	var result DBDelWorkspaceResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, name)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// DBSetWorkspace is used to set the current workspace.
func (msf *MSFRPC) DBSetWorkspace(ctx context.Context, name string) error {
	request := DBSetWorkspaceRequest{
		Method: MethodDBSetWorkspace,
		Token:  msf.GetToken(),
		Name:   name,
	}
	var result DBSetWorkspaceResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, name)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}
