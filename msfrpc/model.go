package msfrpc

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"
	"github.com/vmihailenco/msgpack/v4/codes"
)

// reference:
// https://metasploit.help.rapid7.com/docs/standard-api-methods-reference
// https://rapid7.github.io/metasploit-framework/api/Msf/RPC.html

// errorCode maybe uint16 or string, so need use custom msgpack decoder.
type errorCode uint64

func (e *errorCode) DecodeMsgpack(decoder *msgpack.Decoder) error {
	code, err := decoder.PeekCode()
	if err != nil {
		return err
	}
	switch code {
	case codes.Uint16:
		val, err := decoder.DecodeUint16()
		if err != nil {
			return err
		}
		*e = errorCode(val)
	case codes.Bin8:
		str, err := decoder.DecodeString()
		if err != nil {
			return err
		}
		val, err := strconv.Atoi(str)
		if err != nil {
			return err
		}
		*e = errorCode(val)
	default:
		return errors.Errorf("unknown code: %x", code)
	}
	return nil
}

// MSFError is an error about Metasploit RPC.
type MSFError struct {
	Err            bool      `msgpack:"error"`
	ErrorClass     string    `msgpack:"error_class"`
	ErrorString    string    `msgpack:"error_string"`
	ErrorBacktrace []string  `msgpack:"error_backtrace"`
	ErrorMessage   string    `msgpack:"error_message"`
	ErrorCode      errorCode `msgpack:"error_code"`
}

func (err *MSFError) Error() string {
	return err.ErrorMessage
}

// about errors
const (
	ErrInvalidToken           = "Invalid Authentication Token"
	ErrInvalidTokenFriendly   = "invalid authentication token"
	ErrInvalidWorkspace       = "Invalid workspace"
	ErrInvalidWorkspaceFormat = "workspace %s doesn't exist"
	ErrDBNotLoaded            = "Database Not Loaded"
	ErrDBNotLoadedFriendly    = "database not loaded"
	ErrDBActiveRecord         = "ActiveRecord::ConnectionNotEstablished"
	ErrDBActiveRecordFriendly = "connection not established"
)

// ------------------------------------------about methods-----------------------------------------
const (
	MethodAuthLogin         = "auth.login"
	MethodAuthLogout        = "auth.logout"
	MethodAuthTokenList     = "auth.token_list"     // #nosec
	MethodAuthTokenGenerate = "auth.token_generate" // #nosec
	MethodAuthTokenAdd      = "auth.token_add"      // #nosec
	MethodAuthTokenRemove   = "auth.token_remove"   // #nosec

	MethodCoreModuleStats   = "core.module_stats"
	MethodCoreAddModulePath = "core.add_module_path"
	MethodCoreReloadModules = "core.reload_modules"
	MethodCoreThreadList    = "core.thread_list"
	MethodCoreThreadKill    = "core.thread_kill"
	MethodCoreSetG          = "core.setg"
	MethodCoreUnsetG        = "core.unsetg"
	MethodCoreGetG          = "core.getg"
	MethodCoreSave          = "core.save"
	MethodCoreVersion       = "core.version"

	MethodDBConnect          = "db.connect"
	MethodDBDisconnect       = "db.disconnect"
	MethodDBStatus           = "db.status"
	MethodDBReportHost       = "db.report_host"
	MethodDBHosts            = "db.hosts"
	MethodDBGetHost          = "db.get_host"
	MethodDBDelHost          = "db.del_host"
	MethodDBReportService    = "db.report_service"
	MethodDBServices         = "db.services"
	MethodDBGetService       = "db.get_service"
	MethodDBDelService       = "db.del_service"
	MethodDBCreateCred       = "db.create_cracked_credential"
	MethodDBCreds            = "db.creds"
	MethodDBDelCreds         = "db.del_creds"
	MethodDBReportLoot       = "db.report_loot"
	MethodDBLoots            = "db.loots"
	MethodDBWorkspaces       = "db.workspaces"
	MethodDBGetWorkspace     = "db.get_workspace"
	MethodDBAddWorkspace     = "db.add_workspace"
	MethodDBDelWorkspace     = "db.del_workspace"
	MethodDBSetWorkspace     = "db.set_workspace"
	MethodDBCurrentWorkspace = "db.current_workspace"
	MethodDBEvents           = "db.events"
	MethodDBImportData       = "db.import_data"

	// MethodDBReportNote   = "db.report_note"
	// MethodDBNotes        = "db.notes"
	// MethodDBGetNote      = "db.get_note"
	// MethodDBDelNote      = "db.del_note"
	// MethodDBReportVuln   = "db.report_vuln"
	// MethodDBVulns        = "db.vulns"
	// MethodDBGetVuln      = "db.get_vuln"
	// MethodDBDelVuln      = "db.del_vuln"
	// MethodDBGetRef       = "db.get_ref"
	// MethodDBReportClient = "db.report_client"
	// MethodDBClients      = "db.clients"
	// MethodDBGetClient    = "db.get_client"
	// MethodDBDelClient    = "db.del_client"

	MethodConsoleList          = "console.list"
	MethodConsoleCreate        = "console.create"
	MethodConsoleDestroy       = "console.destroy"
	MethodConsoleRead          = "console.read"
	MethodConsoleWrite         = "console.write"
	MethodConsoleSessionDetach = "console.session_detach"
	MethodConsoleSessionKill   = "console.session_kill"

	MethodPluginLoad   = "plugin.load"
	MethodPluginUnload = "plugin.unload"
	MethodPluginLoaded = "plugin.loaded"

	MethodModuleExploits                        = "module.exploits"
	MethodModuleAuxiliary                       = "module.auxiliary"
	MethodModulePost                            = "module.post"
	MethodModulePayloads                        = "module.payloads"
	MethodModuleEncoders                        = "module.encoders"
	MethodModuleNops                            = "module.nops"
	MethodModuleEvasion                         = "module.evasion"
	MethodModuleInfo                            = "module.info"
	MethodModuleOptions                         = "module.options"
	MethodModuleCompatiblePayloads              = "module.compatible_payloads"
	MethodModuleTargetCompatiblePayloads        = "module.target_compatible_payloads"
	MethodModuleCompatibleSessions              = "module.compatible_sessions"
	MethodModuleCompatibleEvasionPayloads       = "module.compatible_evasion_payloads"
	MethodModuleTargetCompatibleEvasionPayloads = "module.target_compatible_evasion_payloads"
	MethodModuleEncodeFormats                   = "module.encode_formats"
	MethodModuleExecutableFormats               = "module.executable_formats"
	MethodModuleTransformFormats                = "module.transform_formats"
	MethodModuleEncryptionFormats               = "module.encryption_formats"
	MethodModulePlatforms                       = "module.platforms"
	MethodModuleArchitectures                   = "module.architectures"
	MethodModuleEncode                          = "module.encode"
	MethodModuleExecute                         = "module.execute"
	MethodModuleCheck                           = "module.check"
	MethodModuleRunningStats                    = "module.running_stats"

	MethodJobList = "job.list"
	MethodJobInfo = "job.info"
	MethodJobStop = "job.stop"

	MethodSessionList                     = "session.list"
	MethodSessionStop                     = "session.stop"
	MethodSessionShellRead                = "session.shell_read"
	MethodSessionShellWrite               = "session.shell_write"
	MethodSessionMeterpreterRead          = "session.meterpreter_read"
	MethodSessionMeterpreterWrite         = "session.meterpreter_write"
	MethodSessionMeterpreterSessionDetach = "session.meterpreter_session_detach"
	MethodSessionMeterpreterSessionKill   = "session.meterpreter_session_kill"
	MethodSessionMeterpreterRunSingle     = "session.meterpreter_run_single"
	MethodSessionCompatibleModules        = "session.compatible_modules"
)

// structTag is used to xreflect.StructureToMap()
const structTag = "msgpack"

// --------------------------------------about authentication--------------------------------------

// AuthLoginRequest is used to login and get token.
type AuthLoginRequest struct {
	Method   string
	Username string
	Password string
}

// AuthLoginResult is the result about login.
type AuthLoginResult struct {
	Result string `msgpack:"result"`
	Token  string `msgpack:"token"`
	MSFError
}

// AuthLogoutRequest is used to delete token.
type AuthLogoutRequest struct {
	Method      string
	Token       string
	LogoutToken string // will be deleted
}

// AuthLogoutResult is the result about logout.
type AuthLogoutResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// AuthTokenListRequest is used to list tokens.
type AuthTokenListRequest struct {
	Method string
	Token  string
}

// AuthTokenListResult is the result about token list.
type AuthTokenListResult struct {
	Tokens []string `msgpack:"tokens"`
	MSFError
}

// AuthTokenGenerateRequest is used to generate token.
type AuthTokenGenerateRequest struct {
	Method string
	Token  string
}

// AuthTokenGenerateResult is the result about generate token.
type AuthTokenGenerateResult struct {
	Result string `msgpack:"result"`
	Token  string `msgpack:"token"`
	MSFError
}

// AuthTokenAddRequest is used to add token.
type AuthTokenAddRequest struct {
	Method   string
	Token    string
	NewToken string
}

// AuthTokenAddResult is the result about add token.
type AuthTokenAddResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// AuthTokenRemoveRequest is used to remove token.
type AuthTokenRemoveRequest struct {
	Method           string
	Token            string
	TokenToBeRemoved string
}

// AuthTokenRemoveResult is the result about remove token.
type AuthTokenRemoveResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// -------------------------------------------about core-------------------------------------------

// CoreModuleStatsRequest is used to get module status.
type CoreModuleStatsRequest struct {
	Method string
	Token  string
}

// CoreModuleStatsResult is the result about module status.
type CoreModuleStatsResult struct {
	Exploit   int `msgpack:"exploits"`
	Auxiliary int `msgpack:"auxiliary"`
	Post      int `msgpack:"post"`
	Payload   int `msgpack:"payloads"`
	Encoder   int `msgpack:"encoders"`
	Nop       int `msgpack:"nops"`
	MSFError
}

// CoreAddModulePathRequest is used to add module.
type CoreAddModulePathRequest struct {
	Method string
	Token  string
	Path   string
}

// CoreAddModulePathResult is the result about add module.
type CoreAddModulePathResult struct {
	Exploit   int `msgpack:"exploits"`
	Auxiliary int `msgpack:"auxiliary"`
	Post      int `msgpack:"post"`
	Payload   int `msgpack:"payloads"`
	Encoder   int `msgpack:"encoders"`
	Nop       int `msgpack:"nops"`
	MSFError
}

// CoreReloadModulesRequest is used to reload modules.
type CoreReloadModulesRequest struct {
	Method string
	Token  string
}

// CoreReloadModulesResult is the result about reload modules.
type CoreReloadModulesResult struct {
	Exploit   int `msgpack:"exploits"`
	Auxiliary int `msgpack:"auxiliary"`
	Post      int `msgpack:"post"`
	Payload   int `msgpack:"payloads"`
	Encoder   int `msgpack:"encoders"`
	Nop       int `msgpack:"nops"`
	MSFError
}

// CoreThreadListRequest is used to get thread list.
type CoreThreadListRequest struct {
	Method string
	Token  string
}

// CoreThreadInfo contains the thread information.
type CoreThreadInfo struct {
	Status   string `msgpack:"status"`
	Critical bool   `msgpack:"critical"`
	Name     string `msgpack:"name"`
	Started  string `msgpack:"started"`
}

// CoreThreadKillRequest is used to kill thread by ID.
type CoreThreadKillRequest struct {
	Method string
	Token  string
	ID     uint64
}

// CoreThreadKillResult is the result about kill thread.
type CoreThreadKillResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// CoreSetGRequest is used to set global option.
type CoreSetGRequest struct {
	Method string
	Token  string
	Name   string
	Value  string
}

// CoreSetGResult is the result of set global option.
type CoreSetGResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// CoreGetGRequest is used to get global option.
type CoreGetGRequest struct {
	Method string
	Token  string
	Name   string
}

// CoreUnsetGRequest is used to unset global option.
type CoreUnsetGRequest struct {
	Method string
	Token  string
	Name   string
}

// CoreUnsetGResult is the result of unset global option.
type CoreUnsetGResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// CoreSaveRequest is used to save current global data store.
type CoreSaveRequest struct {
	Method string
	Token  string
}

// CoreSaveResult is the result of save.
type CoreSaveResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// CoreVersionRequest is used to get version.
type CoreVersionRequest struct {
	Method string
	Token  string
}

// CoreVersionResult contain information the running framework instance,
// the Ruby interpreter, and the RPC protocol version being used.
type CoreVersionResult struct {
	Version string `msgpack:"version"`
	Ruby    string `msgpack:"ruby"`
	API     string `msgpack:"api"`
	MSFError
}

// -----------------------------------------about database-----------------------------------------

// DBConnectRequest is used to connect database.
type DBConnectRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBConnectOptions contains the options about connect database.
type DBConnectOptions struct {
	Driver   string
	Host     string
	Port     uint64
	Username string
	Password string
	Database string
	Other    map[string]interface{}
}

func (opts *DBConnectOptions) toMap() map[string]interface{} {
	m := map[string]interface{}{
		"driver":   opts.Driver,
		"host":     opts.Host,
		"port":     opts.Port,
		"username": opts.Username,
		"password": opts.Password,
		"database": opts.Database,
	}
	for key, value := range opts.Other {
		m[key] = value
	}
	return m
}

// DBConnectResult is the result of connect database.
type DBConnectResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// DBDisconnectRequest is used to disconnect database.
type DBDisconnectRequest struct {
	Method string
	Token  string
}

// DBDisconnectResult is the result of disconnect database.
type DBDisconnectResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// DBStatusRequest is used to get database status.
type DBStatusRequest struct {
	Method string
	Token  string
}

// DBStatusResult is the result of get database status.
type DBStatusResult struct {
	Driver   string `msgpack:"driver"`
	Database string `msgpack:"db"`
	MSFError
}

// DBReportHostRequest is used to add host to database.
type DBReportHostRequest struct {
	Method string
	Token  string
	Host   map[string]interface{}
}

// DBReportHost contains information about report host.
type DBReportHost struct {
	Workspace     string `msgpack:"workspace"`
	Name          string `msgpack:"name"`
	Host          string `msgpack:"host"`
	MAC           string `msgpack:"mac"`
	OSName        string `msgpack:"os_name"`
	OSFlavor      string `msgpack:"os_flavor"`
	OSServicePack string `msgpack:"os_sp"`
	OSLanguage    string `msgpack:"os_lang"`
	Architecture  string `msgpack:"arch"`
	State         string `msgpack:"state"`
	Scope         string `msgpack:"scope"`
	VirtualHost   string `msgpack:"virtual_host"`
}

// DBReportHostResult is the result of add host to database.
type DBReportHostResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// DBHostsRequest is used to get hosts in database.
type DBHostsRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBHostsResult is the result of get hosts.
type DBHostsResult struct {
	Hosts []*DBHost `msgpack:"hosts"`
	MSFError
}

// DBHost contains host information.
type DBHost struct {
	Name          string `msgpack:"name"`
	Address       string `msgpack:"address"`
	MAC           string `msgpack:"mac"`
	OSName        string `msgpack:"os_name"`
	OSFlavor      string `msgpack:"os_flavor"`
	OSServicePack string `msgpack:"os_sp"`
	OSLanguage    string `msgpack:"os_lang"`
	Purpose       string `msgpack:"purpose"`
	Information   string `msgpack:"info"`
	State         string `msgpack:"state"`
	CreatedAt     int64  `msgpack:"created_at"`
	UpdateAt      int64  `msgpack:"updated_at"`
}

// DBGetHostRequest is used to get host information.
type DBGetHostRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBGetHostResult is the result of get host information.
type DBGetHostResult struct {
	Host []*DBHost `msgpack:"host"`
	MSFError
}

// DBDelHostRequest is used to delete hosts.
type DBDelHostRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBDelHostResult is the result of delete hosts.
type DBDelHostResult struct {
	Result  string   `msgpack:"result"`
	Deleted []string `msgpack:"deleted"`
	MSFError
}

// DBReportServiceRequest is used to add service to database.
type DBReportServiceRequest struct {
	Method  string
	Token   string
	Service map[string]interface{}
}

// DBReportService contains information about service.
type DBReportService struct {
	Workspace string `msgpack:"workspace"`
	Host      string `msgpack:"host"`
	Port      string `msgpack:"port"`
	Protocol  string `msgpack:"proto"`
	Name      string `msgpack:"name"`
}

// DBReportServiceResult is the result of add service to database.
type DBReportServiceResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// DBServicesRequest is used to get services by filter.
type DBServicesRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBServicesOptions contains options about call DBService().
type DBServicesOptions struct {
	Workspace string `msgpack:"workspace"`
	Limit     uint64 `msgpack:"limit"`
	Offset    uint64 `msgpack:"offset"`
	Address   string `msgpack:"address"`
	Port      string `msgpack:"port"`
	Protocol  string `msgpack:"proto"`
	Name      string `msgpack:"name"`
}

// DBServicesResult is the result of get services by filter.
type DBServicesResult struct {
	Services []*DBService `msgpack:"services"`
	MSFError
}

// DBService contains server information.
type DBService struct {
	Host        string `msgpack:"host"`
	Port        uint64 `msgpack:"port"`
	Protocol    string `msgpack:"proto"`
	Name        string `msgpack:"name"`
	State       string `msgpack:"state"`
	Information string `msgpack:"info"`
	CreatedAt   int64  `msgpack:"created_at"`
	UpdateAt    int64  `msgpack:"updated_at"`
}

// DBGetServiceRequest is used to get service.
type DBGetServiceRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBGetServiceOptions contains options about get service.
type DBGetServiceOptions struct {
	Workspace string `msgpack:"workspace"`
	Protocol  string `msgpack:"proto"`
	Port      uint64 `msgpack:"port"`
	Names     string `msgpack:"names"`
}

// DBGetServiceResult is the result of get service.
type DBGetServiceResult struct {
	Service []*DBService `msgpack:"service"`
	MSFError
}

// DBDelServiceRequest is used to delete service.
type DBDelServiceRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBDelServiceOptions contains options about delete service.
type DBDelServiceOptions struct {
	Workspace string   `msgpack:"workspace"`
	Address   string   `msgpack:"address"`
	Addresses []string `msgpack:"addresses"`
	Port      uint64   `msgpack:"port"`
	Protocol  string   `msgpack:"proto"`
}

// DBDelServiceResult is the result of delete service.
type DBDelServiceResult struct {
	Result  string          `msgpack:"result"`
	Deleted []*DBDelService `msgpack:"deleted"`
	MSFError
}

// DBDelService contains information about deleted service.
type DBDelService struct {
	Address  string `msgpack:"address"`
	Port     uint64 `msgpack:"port"`
	Protocol string `msgpack:"proto"`
}

// DBCreateCredentialRequest is used to create credential.
type DBCreateCredentialRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBCreateCredentialOptions contains options about create credential.
type DBCreateCredentialOptions struct {
	OriginType     string `msgpack:"origin_type"`
	Address        string `msgpack:"address"`
	Port           uint64 `msgpack:"port"`
	Protocol       string `msgpack:"protocol"`
	ServiceName    string `msgpack:"service_name"`
	ModuleFullname string `msgpack:"module_fullname"`
	WorkspaceID    uint64 `msgpack:"workspace_id"`
	Username       string `msgpack:"username"`
	PrivateType    string `msgpack:"private_type"`
	PrivateData    string `msgpack:"private_data"`
}

// DBCreateCredentialResult is the result of create credential.
type DBCreateCredentialResult struct {
	Host        string `msgpack:"host"`
	Username    string `msgpack:"username"`
	PrivateType string `msgpack:"private_type"`
	Private     string `msgpack:"private"`
	RealmValue  string `msgpack:"realm_value"`
	RealmKey    string `msgpack:"realm_key"`
	ServiceName string `msgpack:"sname"`
	Status      string `msgpack:"status"`
	MSFError
}

// DBWorkspacesRequest is used to get all workspaces.
type DBWorkspacesRequest struct {
	Method string
	Token  string
}

// DBWorkspacesResult is the result of get all workspaces.
type DBWorkspacesResult struct {
	Workspaces []*DBWorkspace `msgpack:"workspaces"`
	MSFError
}

// DBWorkspace contains information about workspace.
type DBWorkspace struct {
	ID        uint64 `msgpack:"id"`
	Name      string `msgpack:"name"`
	CreatedAt int64  `msgpack:"created_at"`
	UpdateAt  int64  `msgpack:"updated_at"`
}

// DBGetWorkspaceRequest is used to get workspace by name.
type DBGetWorkspaceRequest struct {
	Method string
	Token  string
	Name   string
}

// DBGetWorkspaceResult is the result of get all workspaces.
type DBGetWorkspaceResult struct {
	Workspace []*DBWorkspace `msgpack:"workspace"`
	MSFError
}

// DBAddWorkspaceRequest is used to add workspace.
type DBAddWorkspaceRequest struct {
	Method string
	Token  string
	Name   string
}

// DBAddWorkspaceResult is the result of add workspace.
type DBAddWorkspaceResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// DBDelWorkspaceRequest is used to delete workspace by name.
type DBDelWorkspaceRequest struct {
	Method string
	Token  string
	Name   string
}

// DBDelWorkspaceResult is the result of delete workspace by name.
type DBDelWorkspaceResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// ------------------------------------------about console-----------------------------------------

// ConsoleListRequest is used to list console.
type ConsoleListRequest struct {
	Method string
	Token  string
}

// ConsoleListResult is the result of list console.
type ConsoleListResult struct {
	Consoles []*ConsoleInfo `msgpack:"consoles"`
	MSFError
}

// ConsoleInfo include console information.
type ConsoleInfo struct {
	ID     string `msgpack:"id"`
	Prompt string `msgpack:"prompt"`
	Busy   bool   `msgpack:"busy"`
}

// ConsoleCreateRequest is used to create a console.
type ConsoleCreateRequest struct {
	Method string
	Token  string
}

// ConsoleCreateResult is the result of create console.
type ConsoleCreateResult struct {
	ID     string `msgpack:"id"`
	Prompt string `msgpack:"prompt"`
	Busy   bool   `msgpack:"busy"`
	MSFError
}

// ConsoleDestroyRequest is used to destroy a console.
type ConsoleDestroyRequest struct {
	Method string
	Token  string
	ID     string
}

// ConsoleDestroyResult is the result about destroy console.
type ConsoleDestroyResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// ConsoleWriteRequest is used to write data to a special console.
type ConsoleWriteRequest struct {
	Method string
	Token  string
	ID     string
	Data   string
}

// ConsoleWriteResult is the result about write.
type ConsoleWriteResult struct {
	Wrote  uint64 `msgpack:"wrote"`
	Result string `msgpack:"result"`
	MSFError
}

// ConsoleReadRequest is used to read data from a special console.
type ConsoleReadRequest struct {
	Method string
	Token  string
	ID     string
}

// ConsoleReadResult is the result about read.
type ConsoleReadResult struct {
	Data   string `msgpack:"data"`
	Prompt string `msgpack:"prompt"`
	Busy   bool   `msgpack:"busy"`
	Result string `msgpack:"result"`
	MSFError
}

// ConsoleSessionDetachRequest is used to background an interactive session.
type ConsoleSessionDetachRequest struct {
	Method string
	Token  string
	ID     string
}

// ConsoleSessionDetachResult is the result of detach session.
type ConsoleSessionDetachResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// ConsoleSessionKillRequest is used to kill an interactive session.
type ConsoleSessionKillRequest struct {
	Method string
	Token  string
	ID     string
}

// ConsoleSessionKillResult is the result of kill session.
type ConsoleSessionKillResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// ------------------------------------------about plugin------------------------------------------

// PluginLoadRequest is used to load plugin.
type PluginLoadRequest struct {
	Method  string
	Token   string
	Name    string
	Options map[string]string
}

// PluginLoadResult is the result of load plugin.
type PluginLoadResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// PluginUnloadRequest is used to unload plugin.
type PluginUnloadRequest struct {
	Method string
	Token  string
	Name   string
}

// PluginUnloadResult is the result of unload plugin.
type PluginUnloadResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// PluginLoadedRequest is used to enumerate all currently loaded plugins.
type PluginLoadedRequest struct {
	Method string
	Token  string
}

// PluginLoadedResult is the plugin name list.
type PluginLoadedResult struct {
	Plugins []string `msgpack:"plugins"`
	MSFError
}

// ------------------------------------------about module------------------------------------------

// ModuleExploitsRequest is used to get all modules about exploit.
type ModuleExploitsRequest struct {
	Method string
	Token  string
}

// ModuleExploitsResult is the result about get exploit modules.
type ModuleExploitsResult struct {
	Modules []string `msgpack:"modules"`
	MSFError
}

// ModuleAuxiliaryRequest is used to get all modules about auxiliary.
type ModuleAuxiliaryRequest struct {
	Method string
	Token  string
}

// ModuleAuxiliaryResult is the result of get auxiliary modules.
type ModuleAuxiliaryResult struct {
	Modules []string `msgpack:"modules"`
	MSFError
}

// ModulePostRequest is used to get all modules about post.
type ModulePostRequest struct {
	Method string
	Token  string
}

// ModulePostResult is the result of get post modules.
type ModulePostResult struct {
	Modules []string `msgpack:"modules"`
	MSFError
}

// ModulePayloadsRequest is used to get all modules about payload.
type ModulePayloadsRequest struct {
	Method string
	Token  string
}

// ModulePayloadsResult is the result about get payload modules.
type ModulePayloadsResult struct {
	Modules []string `msgpack:"modules"`
	MSFError
}

// ModuleEncodersRequest is used to get all modules about encoders.
type ModuleEncodersRequest struct {
	Method string
	Token  string
}

// ModuleEncodersResult is the result about get encoder modules.
type ModuleEncodersResult struct {
	Modules []string `msgpack:"modules"`
	MSFError
}

// ModuleNopsRequest is used to get all modules about nop.
type ModuleNopsRequest struct {
	Method string
	Token  string
}

// ModuleNopsResult is the result about get nop modules.
type ModuleNopsResult struct {
	Modules []string `msgpack:"modules"`
	MSFError
}

// ModuleEvasionRequest is used to get all modules about evasion.
type ModuleEvasionRequest struct {
	Method string
	Token  string
}

// ModuleEvasionResult is the result about get evasion modules.
type ModuleEvasionResult struct {
	Modules []string `msgpack:"modules"`
	MSFError
}

// ModuleInfoRequest is used to get module's information.
type ModuleInfoRequest struct {
	Method string
	Token  string
	Type   string
	Name   string
}

// ModuleInfoResult is the result about get module's information.
type ModuleInfoResult struct {
	Type           string                   `msgpack:"type"`
	Name           string                   `msgpack:"name"`
	FullName       string                   `msgpack:"fullname"`
	Rank           string                   `msgpack:"rank"`
	DisclosureDate string                   `msgpack:"disclosuredate"`
	Description    string                   `msgpack:"description"`
	License        string                   `msgpack:"license"`
	Filepath       string                   `msgpack:"filepath"`
	Arch           []string                 `msgpack:"arch"`
	Platform       []string                 `msgpack:"platform"`
	Authors        []string                 `msgpack:"authors"`
	Privileged     bool                     `msgpack:"privileged"`
	References     []string                 `msgpack:"references"`
	Targets        map[uint64]string        `msgpack:"targets"`
	DefaultTarget  uint64                   `msgpack:"default_target"`
	Stance         string                   `msgpack:"stance"`
	Options        map[string]*ModuleOption `msgpack:"options"`
	MSFError
}

// ModuleOption contains modules information about options.
type ModuleOption struct {
	Type        string      `msgpack:"type"`
	Required    bool        `msgpack:"required"`
	Advanced    bool        `msgpack:"advanced"`
	Description string      `msgpack:"desc"`
	Default     interface{} `msgpack:"default"`
}

// ModuleOptionsRequest is used to get module options.
type ModuleOptionsRequest struct {
	Method string
	Token  string
	Type   string
	Name   string
}

// ModuleSpecialOption contains modules options for ModuleOptionsRequest.
type ModuleSpecialOption struct {
	Type        string        `msgpack:"type"`
	Required    bool          `msgpack:"required"`
	Advanced    bool          `msgpack:"advanced"`
	Evasion     bool          `msgpack:"evasion"`
	Description string        `msgpack:"desc"`
	Default     interface{}   `msgpack:"default"`
	Enums       []interface{} `msgpack:"enums"`
}

// ModuleCompatiblePayloadsRequest is used to get compatible payloads.
type ModuleCompatiblePayloadsRequest struct {
	Method string
	Token  string
	Name   string
}

// ModuleCompatiblePayloadsResult is the result of get compatible payloads.
type ModuleCompatiblePayloadsResult struct {
	Payloads []string `msgpack:"payloads"`
	MSFError
}

// ModuleTargetCompatiblePayloadsRequest is used to get target compatible payloads.
type ModuleTargetCompatiblePayloadsRequest struct {
	Method string
	Token  string
	Name   string
	Target uint64
}

// ModuleTargetCompatiblePayloadsResult is the result of get target compatible payloads.
type ModuleTargetCompatiblePayloadsResult struct {
	Payloads []string `msgpack:"payloads"`
	MSFError
}

// ModuleCompatibleSessionsRequest is used to get compatible sessions for post module.
type ModuleCompatibleSessionsRequest struct {
	Method string
	Token  string
	Name   string
}

// ModuleCompatibleSessionsResult is the result about get compatible sessions.
type ModuleCompatibleSessionsResult struct {
	Sessions []string `msgpack:"sessions"`
	MSFError
}

// ModuleCompatibleEvasionPayloadsRequest is used to get compatible evasion payloads.
type ModuleCompatibleEvasionPayloadsRequest struct {
	Method string
	Token  string
	Name   string
}

// ModuleCompatibleEvasionPayloadsResult is the result about get compatible evasion payloads.
type ModuleCompatibleEvasionPayloadsResult struct {
	Payloads []string `msgpack:"payloads"`
	MSFError
}

// ModuleTargetCompatibleEvasionPayloadsRequest is used to get target compatible evasion payloads.
type ModuleTargetCompatibleEvasionPayloadsRequest struct {
	Method string
	Token  string
	Name   string
	Target uint64
}

// ModuleTargetCompatibleEvasionPayloadsResult is the result about get target
// compatible evasion payloads.
type ModuleTargetCompatibleEvasionPayloadsResult struct {
	Payloads []string `msgpack:"payloads"`
	MSFError
}

// ModuleEncodeFormatsRequest is used to get encode format names.
type ModuleEncodeFormatsRequest struct {
	Method string
	Token  string
}

// ModuleExecutableFormatsRequest is used to get executable format names.
type ModuleExecutableFormatsRequest struct {
	Method string
	Token  string
}

// ModuleTransformFormatsRequest is used to get transform format names.
type ModuleTransformFormatsRequest struct {
	Method string
	Token  string
}

// ModuleEncryptionFormatsRequest is used to get encryption format names.
type ModuleEncryptionFormatsRequest struct {
	Method string
	Token  string
}

// ModulePlatformsRequest is used to get platforms.
type ModulePlatformsRequest struct {
	Method string
	Token  string
}

// ModuleArchitecturesRequest is used to get architectures.
type ModuleArchitecturesRequest struct {
	Method string
	Token  string
}

// ModuleEncodeRequest is used to encode data with an encoder.
type ModuleEncodeRequest struct {
	Method  string
	Token   string
	Data    string
	Encoder string // like "x86/single_byte"
	Options map[string]interface{}
}

// ModuleEncodeOptions contains module encode options.
type ModuleEncodeOptions struct {
	Format       string
	BadChars     string
	Platform     string
	Arch         string
	EncodeCount  uint64
	Inject       bool
	AltEXE       string
	EXEDir       string
	AddShellcode string
}

func (opts *ModuleEncodeOptions) toMap() map[string]interface{} {
	m := map[string]interface{}{
		"format":   opts.Format,
		"badchars": opts.BadChars,
		"platform": opts.Platform,
		"arch":     opts.Arch,
		"ecount":   opts.EncodeCount,
		"inject":   opts.Inject,
		"altexe":   opts.AltEXE,
		"exedir":   opts.EXEDir,
	}
	if opts.EncodeCount > 0 {
		m["ecount"] = opts.EncodeCount
	}
	// TODO [external] msfrpcd bug about ModuleEncode
	// file: lib/msf/core/rpc/v10/rpc_module.rb
	//
	//  if options['addshellcode']
	//      buf = Msf::Util::EXE.win32_rwx_exec_thread(buf,0,'end')
	//      file = ::File.new(options['addshellcode'])
	//      file.binmode
	//      buf << file.read
	//      file.close

	// our golang code
	// if opts.AddShellcode != "" {
	//     m["addshellcode"] = opts.AddShellcode
	// }
	return m
}

// ModuleEncodeResult is the result about encode.
type ModuleEncodeResult struct {
	Encoded string `msgpack:"encoded"`
	MSFError
}

// ModuleExecuteRequest is used to execute a module.
type ModuleExecuteRequest struct {
	Method  string
	Token   string
	Type    string
	Name    string
	Options map[string]interface{}
}

// ModuleExecuteOptions is used to generate payload.
type ModuleExecuteOptions struct {
	BadChars            string
	Format              string
	ForceEncoding       bool
	Template            string
	Platform            string
	KeepTemplateWorking bool
	NopSledSize         uint64
	Iterations          uint64
	DataStore           map[string]interface{}
}

// NewModuleExecuteOptions is used to create a module execute options.
func NewModuleExecuteOptions() *ModuleExecuteOptions {
	return &ModuleExecuteOptions{DataStore: make(map[string]interface{})}
}

func (opts *ModuleExecuteOptions) toMap() map[string]interface{} {
	m := map[string]interface{}{
		"BadChars":            opts.BadChars,
		"Format":              opts.Format,
		"ForceEncoding":       opts.ForceEncoding,
		"Template":            opts.Template,
		"Platform":            opts.Platform,
		"KeepTemplateWorking": opts.KeepTemplateWorking,
		"NopSledSize":         opts.NopSledSize,
	}
	if opts.Iterations > 0 {
		m["Iterations"] = opts.Iterations
	}
	for key, value := range opts.DataStore {
		m[key] = value
	}
	return m
}

// ModuleExecuteResult is the result of execute a module.
type ModuleExecuteResult struct {
	JobID   uint64 `msgpack:"job_id"`
	UUID    string `msgpack:"uuid"`
	Payload string `msgpack:"payload"`
	MSFError
}

// ModuleCheckRequest is used to check exploit and auxiliary module.
type ModuleCheckRequest struct {
	Method  string
	Token   string
	Type    string
	Name    string
	Options map[string]interface{}
}

// ModuleCheckResult is the result of check module.
type ModuleCheckResult struct {
	JobID uint64 `msgpack:"job_id"`
	UUID  string `msgpack:"uuid"`
	MSFError
}

// ModuleRunningStatsRequest is used to get the currently running
// module stats in each state.
type ModuleRunningStatsRequest struct {
	Method string
	Token  string
}

// ModuleRunningStatsResult is the result of get running stats.
type ModuleRunningStatsResult struct {
	Waiting []string `msgpack:"waiting"`
	Running []string `msgpack:"running"`
	Results []string `msgpack:"results"`
	MSFError
}

// -------------------------------------------about job--------------------------------------------

// JobListRequest is used to list jobs.
type JobListRequest struct {
	Method string
	Token  string
}

// JobInfoRequest is used to get job information by job id.
type JobInfoRequest struct {
	Method string
	Token  string
	ID     string
}

// JobInfoResult is the result of get job information.
type JobInfoResult struct {
	JobID     uint64                 `msgpack:"jid"`
	Name      string                 `msgpack:"name"`
	StartTime uint64                 `msgpack:"start_time"`
	DataStore map[string]interface{} `msgpack:"datastore"`
	MSFError
}

// JobStopRequest is used to stop job.
type JobStopRequest struct {
	Method string
	Token  string
	ID     string
}

// JobStopResult is the result of stop job.
type JobStopResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// -----------------------------------------about session------------------------------------------

// SessionListRequest is used to get session list.
type SessionListRequest struct {
	Method string
	Token  string
}

// SessionInfo contains the session information.
type SessionInfo struct {
	Type         string `msgpack:"type"`
	ViaExploit   string `msgpack:"via_exploit"`
	ViaPayload   string `msgpack:"via_payload"`
	TunnelLocal  string `msgpack:"tunnel_local"`
	TunnelPeer   string `msgpack:"tunnel_peer"`
	SessionHost  string `msgpack:"session_host"`
	SessionPort  uint64 `msgpack:"session_port"`
	TargetHost   string `msgpack:"target_host"`
	Routes       string `msgpack:"routes"`
	Username     string `msgpack:"username"`
	Architecture string `msgpack:"arch"`
	Platform     string `msgpack:"platform"`
	Description  string `msgpack:"desc"`
	Information  string `msgpack:"info"`
	Workspace    string `msgpack:"workspace"`
	UUID         string `msgpack:"uuid"`
	ExploitUUID  string `msgpack:"exploit_uuid"`
}

// SessionStopRequest is used to stop a session.
type SessionStopRequest struct {
	Method string
	Token  string
	ID     uint64
}

// SessionStopResult is the result of stop session.
type SessionStopResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// SessionShellReadRequest is used to read data from a shell.
type SessionShellReadRequest struct {
	Method  string
	Token   string
	ID      uint64
	Pointer uint64
}

// SessionShellReadResult is the result of read shell.
type SessionShellReadResult struct {
	Seq  uint64 `msgpack:"seq"`
	Data string `msgpack:"data"`
	MSFError
}

// SessionShellWriteRequest is used to write data to a shell.
type SessionShellWriteRequest struct {
	Method string
	Token  string
	ID     uint64
	Data   string
}

// SessionShellWriteResult is the result of write shell.
type SessionShellWriteResult struct {
	WriteCount string `msgpack:"write_count"`
	MSFError
}

// SessionMeterpreterReadRequest is used to read data from a meterpreter shell.
type SessionMeterpreterReadRequest struct {
	Method string
	Token  string
	ID     uint64
}

// SessionMeterpreterReadResult is the result of read meterpreter shell.
type SessionMeterpreterReadResult struct {
	Data string `msgpack:"data"`
	MSFError
}

// SessionMeterpreterWriteRequest is used to write data to a meterpreter shell.
type SessionMeterpreterWriteRequest struct {
	Method string
	Token  string
	ID     uint64
	Data   string
}

// SessionMeterpreterWriteResult is the result of write meterpreter shell.
type SessionMeterpreterWriteResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// SessionMeterpreterSessionDetachRequest is used to detach with a meterpreter session.
type SessionMeterpreterSessionDetachRequest struct {
	Method string
	Token  string
	ID     uint64
}

// SessionMeterpreterSessionDetachResult is the result of detach a meterpreter session.
type SessionMeterpreterSessionDetachResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// SessionMeterpreterSessionKillRequest is used to kill a meterpreter session.
type SessionMeterpreterSessionKillRequest struct {
	Method string
	Token  string
	ID     uint64
}

// SessionMeterpreterSessionKillResult is the result of kill meterpreter session.
type SessionMeterpreterSessionKillResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// SessionMeterpreterRunSingleRequest is used to run single post module.
type SessionMeterpreterRunSingleRequest struct {
	Method  string
	Token   string
	ID      uint64
	Command string
}

// SessionMeterpreterRunSingleResult is the result of run single post module.
type SessionMeterpreterRunSingleResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// SessionCompatibleModulesRequest is used to get compatible modules.
type SessionCompatibleModulesRequest struct {
	Method string
	Token  string
	ID     uint64
}

// SessionCompatibleModulesResult is the result of get compatible modules.
type SessionCompatibleModulesResult struct {
	Modules []string `msgpack:"modules"`
	MSFError
}
