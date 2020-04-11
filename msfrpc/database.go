package msfrpc

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"project/internal/xreflect"
)

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
	return msf.DBAddWorkspace(ctx, defaultWorkspace)
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
	hostCp := *host
	if hostCp.Workspace == "" {
		hostCp.Workspace = defaultWorkspace
	}
	request := DBReportHostRequest{
		Method: MethodDBReportHost,
		Token:  msf.GetToken(),
		Host:   xreflect.StructureToMap(&hostCp, structTag),
	}
	var result DBReportHostResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, hostCp.Workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
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
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
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
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
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
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	if result.Result != "success" {
		const format = "host: %s doesn't exist in workspace: %s"
		return nil, errors.Errorf(format, address, workspace)
	}
	return result.Deleted, nil
}

// DBReportService is used to add service to database.
func (msf *MSFRPC) DBReportService(ctx context.Context, service *DBReportService) error {
	serviceCp := *service
	if serviceCp.Workspace == "" {
		serviceCp.Workspace = defaultWorkspace
	}
	request := DBReportServiceRequest{
		Method:  MethodDBReportService,
		Token:   msf.GetToken(),
		Service: xreflect.StructureToMap(&serviceCp, structTag),
	}
	var result DBReportServiceResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, serviceCp.Workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// DBServices is used to get services by filter options.
// Must set Protocol in DBServicesOptions.
func (msf *MSFRPC) DBServices(ctx context.Context, opts *DBServicesOptions) ([]*DBService, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBServicesRequest{
		Method:  MethodDBServices,
		Token:   msf.GetToken(),
		Options: xreflect.StructureToMap(&optsCp, structTag),
	}
	var result DBServicesResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, optsCp.Workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
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
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBGetServiceRequest{
		Method:  MethodDBGetService,
		Token:   msf.GetToken(),
		Options: xreflect.StructureToMap(&optsCp, structTag),
	}
	var result DBGetServiceResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, optsCp.Workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
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
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBDelServiceRequest{
		Method:  MethodDBDelService,
		Token:   msf.GetToken(),
		Options: xreflect.StructureToMapWithoutZero(&optsCp, structTag),
	}
	var result DBDelServiceResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, optsCp.Workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	if result.Result != "success" {
		return nil, errors.New("failed to delete service")
	}
	return result.Deleted, nil
}

// DBReportClient is used to add browser client to database.
func (msf *MSFRPC) DBReportClient(ctx context.Context, client *DBReportClient) error {
	clientCp := *client
	if clientCp.Workspace == "" {
		clientCp.Workspace = defaultWorkspace
	}
	request := DBReportClientRequest{
		Method:  MethodDBReportClient,
		Token:   msf.GetToken(),
		Options: xreflect.StructureToMap(&clientCp, structTag),
	}
	var result DBReportClientResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, clientCp.Workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// DBClients is used to get browser clients by filter.
func (msf *MSFRPC) DBClients(ctx context.Context, opts *DBClientsOptions) ([]*DBClient, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBClientsRequest{
		Method:  MethodDBClients,
		Token:   msf.GetToken(),
		Options: xreflect.StructureToMap(&optsCp, structTag),
	}
	var result DBClientsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, optsCp.Workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Clients, nil
}

// DBGetClient is used to get browser client by filter.
func (msf *MSFRPC) DBGetClient(ctx context.Context, opts *DBGetClientOptions) (*DBClient, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBGetClientRequest{
		Method:  MethodDBGetClient,
		Token:   msf.GetToken(),
		Options: xreflect.StructureToMap(&optsCp, structTag),
	}
	var result DBGetClientResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, optsCp.Workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	if len(result.Client) == 0 {
		return nil, errors.Errorf("client: %s doesn't exist", opts.Host)
	}
	return result.Client[0], nil
}

// DBDelClient is used to delete browser client by filter, it wil return deleted browser clients.
func (msf *MSFRPC) DBDelClient(ctx context.Context, opts *DBDelClientOptions) ([]*DBDelClient, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBDelClientRequest{
		Method:  MethodDBDelClient,
		Token:   msf.GetToken(),
		Options: xreflect.StructureToMapWithoutZero(&optsCp, structTag),
	}
	var result DBDelClientResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, optsCp.Workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Deleted, nil
}

// DBCreateCredential is used to create a credential.
func (msf *MSFRPC) DBCreateCredential(
	ctx context.Context,
	opts map[string]interface{},
) (*DBCreateCredentialResult, error) {
	request := DBCreateCredentialRequest{
		Method:  MethodDBCreateCred,
		Token:   msf.GetToken(),
		Options: opts,
	}
	var result DBCreateCredentialResult
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

// DBCreds is used to get all credentials with workspace.
func (msf *MSFRPC) DBCreds(ctx context.Context, workspace string) ([]*DBCred, error) {
	if workspace == "" {
		workspace = defaultWorkspace
	}
	request := DBCredsRequest{
		Method: MethodDBCreds,
		Token:  msf.GetToken(),
		Options: map[string]interface{}{
			"workspace": workspace,
		},
	}
	var result DBCredsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Credentials, nil
}

// DBDelCreds is used to delete credentials with workspace.
func (msf *MSFRPC) DBDelCreds(ctx context.Context, workspace string) ([]*DBDelCred, error) {
	if workspace == "" {
		workspace = defaultWorkspace
	}
	request := DBDelCredsRequest{
		Method: MethodDBDelCreds,
		Token:  msf.GetToken(),
		Options: map[string]interface{}{
			"workspace": workspace,
		},
	}
	var result DBDelCredsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	var creds []*DBDelCred
	for i := 0; i < len(result.Deleted); i++ {
		creds = append(creds, result.Deleted[i].Creds...)
	}
	if len(creds) == 0 {
		return nil, errors.New("failed to delete credentials in workspace: " + workspace)
	}
	return creds, nil
}

// DBReportLoot is used to add a loot to database.
func (msf *MSFRPC) DBReportLoot(ctx context.Context, loot *DBReportLoot) error {
	lootCp := *loot
	if lootCp.Workspace == "" {
		lootCp.Workspace = defaultWorkspace
	}
	request := DBReportLootRequest{
		Method:  MethodDBReportLoot,
		Token:   msf.GetToken(),
		Options: xreflect.StructureToMapWithoutZero(&lootCp, structTag),
	}
	var result DBReportLootResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, lootCp.Workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// DBLoots is used to get loots by filter.
func (msf *MSFRPC) DBLoots(ctx context.Context, opts *DBLootsOptions) ([]*DBLoot, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBLootsRequest{
		Method:  MethodDBLoots,
		Token:   msf.GetToken(),
		Options: xreflect.StructureToMap(&optsCp, structTag),
	}
	var result DBLootsResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, optsCp.Workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Loots, nil
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
		switch result.ErrorMessage {
		case ErrDBNotLoaded:
			result.ErrorMessage = ErrDBNotLoadedFriendly
		case ErrInvalidToken:
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
	if name == "" {
		return nil
	}
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
	if name == "" {
		return nil
	}
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
	if name == "" {
		return nil
	}
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

// DBCurrentWorkspace is used to get the current workspace.
func (msf *MSFRPC) DBCurrentWorkspace(ctx context.Context) (*DBCurrentWorkspaceResult, error) {
	request := DBCurrentWorkspaceRequest{
		Method: MethodDBCurrentWorkspace,
		Token:  msf.GetToken(),
	}
	var result DBCurrentWorkspaceResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrDBNotLoaded:
			result.ErrorMessage = ErrDBNotLoadedFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return &result, nil
}

// DBEvent is used to get framework events.
func (msf *MSFRPC) DBEvent(
	ctx context.Context,
	workspace string,
	limit uint64,
	offset uint64,
) ([]*DBEvent, error) {
	if workspace == "" {
		workspace = defaultWorkspace
	}
	request := DBEventRequest{
		Method: MethodDBEvents,
		Token:  msf.GetToken(),
		Options: map[string]interface{}{
			"workspace": workspace,
			"limit":     limit,
			"offset":    offset,
		},
	}
	var result DBEventResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	// replace []byte to string
	for i := 0; i < len(result.Events); i++ {
		m := result.Events[i].Information
		for key, value := range m {
			if v, ok := value.([]byte); ok {
				m[key] = string(v)
			}
		}
	}
	return result.Events, nil
}

// DBImportData is used to import external data to the database.
func (msf *MSFRPC) DBImportData(ctx context.Context, workspace, data string) error {
	if len(data) == 0 {
		return errors.New("no data")
	}
	if workspace == "" {
		workspace = defaultWorkspace
	}
	request := DBImportDataRequest{
		Method: MethodDBImportData,
		Token:  msf.GetToken(),
		Options: map[string]interface{}{
			"workspace": workspace,
			"data":      data,
		},
	}
	var result DBImportDataResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "Could not automatically determine file type":
			result.ErrorMessage = "invalid file format"
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}
