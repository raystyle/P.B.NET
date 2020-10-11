package msfrpc

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/xreflect"
)

// DBConnect is used to connect database.
func (client *Client) DBConnect(ctx context.Context, opts *DBConnectOptions) error {
	request := DBConnectRequest{
		Method:  MethodDBConnect,
		Token:   client.GetToken(),
		Options: opts.toMap(),
	}
	var result DBConnectResult
	err := client.send(ctx, &request, &result)
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
		return errors.Errorf("failed to connect database: %s", result.Result)
	}
	err = client.DBAddWorkspace(ctx, defaultWorkspace)
	if err != nil {
		return err
	}
	const format = "connected database: %s %s:%d"
	client.logf(logger.Info, format, opts.Driver, opts.Host, opts.Port)
	return nil
}

// DBDisconnect is used to disconnect database.
func (client *Client) DBDisconnect(ctx context.Context) error {
	request := DBDisconnectRequest{
		Method: MethodDBDisconnect,
		Token:  client.GetToken(),
	}
	var result DBDisconnectResult
	err := client.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		if result.ErrorMessage == ErrInvalidToken {
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	client.log(logger.Info, "disconnect database")
	return nil
}

// DBStatus is used to get the database status.
func (client *Client) DBStatus(ctx context.Context) (*DBStatusResult, error) {
	request := DBStatusRequest{
		Method: MethodDBStatus,
		Token:  client.GetToken(),
	}
	var result DBStatusResult
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

// DBReportHost is used to add host to database.
func (client *Client) DBReportHost(ctx context.Context, host *DBReportHost) error {
	hostCp := *host
	if hostCp.Workspace == "" {
		hostCp.Workspace = defaultWorkspace
	}
	request := DBReportHostRequest{
		Method: MethodDBReportHost,
		Token:  client.GetToken(),
		Host:   xreflect.StructureToMap(&hostCp, structTag),
	}
	var result DBReportHostResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBHosts(ctx context.Context, workspace string) ([]*DBHost, error) {
	if workspace == "" {
		workspace = defaultWorkspace
	}
	request := DBHostsRequest{
		Method: MethodDBHosts,
		Token:  client.GetToken(),
		Options: map[string]interface{}{
			"workspace": workspace,
		},
	}
	var result DBHostsResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBGetHost(ctx context.Context, opts *DBGetHostOptions) (*DBHost, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBGetHostRequest{
		Method:  MethodDBGetHost,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMap(&optsCp, structTag),
	}
	var result DBGetHostResult
	err := client.send(ctx, &request, &result)
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
	if len(result.Host) == 0 {
		return nil, errors.Errorf("host: %s doesn't exist", optsCp.Address)
	}
	return result.Host[0], nil
}

// DBDelHost is used to delete host by filters, it will return deleted host.
func (client *Client) DBDelHost(ctx context.Context, opts *DBDelHostOptions) ([]string, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBDelHostRequest{
		Method:  MethodDBDelHost,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMapWithoutZero(&optsCp, structTag),
	}
	var result DBDelHostResult
	err := client.send(ctx, &request, &result)
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
		const format = "host: %s doesn't exist in workspace: %s"
		return nil, errors.Errorf(format, optsCp.Address, optsCp.Workspace)
	}
	return result.Deleted, nil
}

// DBReportService is used to add service to database.
func (client *Client) DBReportService(ctx context.Context, service *DBReportService) error {
	serviceCp := *service
	if serviceCp.Workspace == "" {
		serviceCp.Workspace = defaultWorkspace
	}
	request := DBReportServiceRequest{
		Method:  MethodDBReportService,
		Token:   client.GetToken(),
		Service: xreflect.StructureToMap(&serviceCp, structTag),
	}
	var result DBReportServiceResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBServices(ctx context.Context, opts *DBServicesOptions) ([]*DBService, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBServicesRequest{
		Method:  MethodDBServices,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMap(&optsCp, structTag),
	}
	var result DBServicesResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBGetService(ctx context.Context, opts *DBGetServiceOptions) ([]*DBService, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBGetServiceRequest{
		Method:  MethodDBGetService,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMap(&optsCp, structTag),
	}
	var result DBGetServiceResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBDelService(ctx context.Context, opts *DBDelServiceOptions) ([]*DBDelService, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBDelServiceRequest{
		Method:  MethodDBDelService,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMapWithoutZero(&optsCp, structTag),
	}
	var result DBDelServiceResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBReportClient(ctx context.Context, client *DBReportClient) error {
	clientCp := *client
	if clientCp.Workspace == "" {
		clientCp.Workspace = defaultWorkspace
	}
	request := DBReportClientRequest{
		Method:  MethodDBReportClient,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMap(&clientCp, structTag),
	}
	var result DBReportClientResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBClients(ctx context.Context, opts *DBClientsOptions) ([]*DBClient, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBClientsRequest{
		Method:  MethodDBClients,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMap(&optsCp, structTag),
	}
	var result DBClientsResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBGetClient(ctx context.Context, opts *DBGetClientOptions) (*DBClient, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBGetClientRequest{
		Method:  MethodDBGetClient,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMap(&optsCp, structTag),
	}
	var result DBGetClientResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBDelClient(ctx context.Context, opts *DBDelClientOptions) ([]*DBDelClient, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBDelClientRequest{
		Method:  MethodDBDelClient,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMapWithoutZero(&optsCp, structTag),
	}
	var result DBDelClientResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBCreateCredential(
	ctx context.Context,
	opts *DBCreateCredentialOptions,
) (*DBCreateCredentialResult, error) {
	request := DBCreateCredentialRequest{
		Method:  MethodDBCreateCred,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMapWithoutZero(opts, structTag),
	}
	var result DBCreateCredentialResult
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

// DBCreds is used to get all credentials with workspace.
func (client *Client) DBCreds(ctx context.Context, workspace string) ([]*DBCred, error) {
	if workspace == "" {
		workspace = defaultWorkspace
	}
	request := DBCredsRequest{
		Method: MethodDBCreds,
		Token:  client.GetToken(),
		Options: map[string]interface{}{
			"workspace": workspace,
		},
	}
	var result DBCredsResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBDelCreds(ctx context.Context, workspace string) ([]*DBDelCred, error) {
	if workspace == "" {
		workspace = defaultWorkspace
	}
	request := DBDelCredsRequest{
		Method: MethodDBDelCreds,
		Token:  client.GetToken(),
		Options: map[string]interface{}{
			"workspace": workspace,
		},
	}
	var result DBDelCredsResult
	err := client.send(ctx, &request, &result)
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
	return creds, nil
}

// DBReportLoot is used to add a loot to database.
func (client *Client) DBReportLoot(ctx context.Context, loot *DBReportLoot) error {
	lootCp := *loot
	if lootCp.Workspace == "" {
		lootCp.Workspace = defaultWorkspace
	}
	request := DBReportLootRequest{
		Method:  MethodDBReportLoot,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMapWithoutZero(&lootCp, structTag),
	}
	var result DBReportLootResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBLoots(ctx context.Context, opts *DBLootsOptions) ([]*DBLoot, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBLootsRequest{
		Method:  MethodDBLoots,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMap(&optsCp, structTag),
	}
	var result DBLootsResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBWorkspaces(ctx context.Context) ([]*DBWorkspace, error) {
	request := DBWorkspacesRequest{
		Method: MethodDBWorkspaces,
		Token:  client.GetToken(),
	}
	var result DBWorkspacesResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBGetWorkspace(ctx context.Context, name string) (*DBWorkspace, error) {
	request := DBGetWorkspaceRequest{
		Method: MethodDBGetWorkspace,
		Token:  client.GetToken(),
		Name:   name,
	}
	var result DBGetWorkspaceResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBAddWorkspace(ctx context.Context, name string) error {
	if name == "" {
		return nil
	}
	request := DBAddWorkspaceRequest{
		Method: MethodDBAddWorkspace,
		Token:  client.GetToken(),
		Name:   name,
	}
	var result DBAddWorkspaceResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBDelWorkspace(ctx context.Context, name string) error {
	if name == "" {
		return nil
	}
	request := DBDelWorkspaceRequest{
		Method: MethodDBDelWorkspace,
		Token:  client.GetToken(),
		Name:   name,
	}
	var result DBDelWorkspaceResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBSetWorkspace(ctx context.Context, name string) error {
	if name == "" {
		return nil
	}
	request := DBSetWorkspaceRequest{
		Method: MethodDBSetWorkspace,
		Token:  client.GetToken(),
		Name:   name,
	}
	var result DBSetWorkspaceResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBCurrentWorkspace(ctx context.Context) (*DBCurrentWorkspaceResult, error) {
	request := DBCurrentWorkspaceRequest{
		Method: MethodDBCurrentWorkspace,
		Token:  client.GetToken(),
	}
	var result DBCurrentWorkspaceResult
	err := client.send(ctx, &request, &result)
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
func (client *Client) DBEvent(ctx context.Context, opts *DBEventOptions) ([]*DBEvent, error) {
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBEventRequest{
		Method:  MethodDBEvents,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMapWithoutZero(&optsCp, structTag),
	}
	var result DBEventResult
	err := client.send(ctx, &request, &result)
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
	// replace []byte to string
	for i := 0; i < len(result.Events); i++ {
		m := result.Events[i].Information
		for key, value := range m {
			if v, ok := value.([]byte); ok {
				m[key] = string(v)
			}
		}
		// check data store is exist
		value, ok := m["datastore"]
		if !ok {
			continue
		}
		dataStore := value.(map[string]interface{})
		for key, value := range dataStore {
			if v, ok := value.([]byte); ok {
				dataStore[key] = string(v)
			}
		}
	}
	return result.Events, nil
}

// DBImportData is used to import external data to the database.
func (client *Client) DBImportData(ctx context.Context, opts *DBImportDataOptions) error {
	if len(opts.Data) == 0 {
		return errors.New("no data")
	}
	optsCp := *opts
	if optsCp.Workspace == "" {
		optsCp.Workspace = defaultWorkspace
	}
	request := DBImportDataRequest{
		Method:  MethodDBImportData,
		Token:   client.GetToken(),
		Options: xreflect.StructureToMap(&optsCp, structTag),
	}
	var result DBImportDataResult
	err := client.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		switch result.ErrorMessage {
		case "Could not automatically determine file type":
			result.ErrorMessage = "invalid file format"
		case ErrInvalidWorkspace:
			result.ErrorMessage = fmt.Sprintf(ErrInvalidWorkspaceFormat, optsCp.Workspace)
		case ErrDBActiveRecord:
			result.ErrorMessage = ErrDBActiveRecordFriendly
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}
