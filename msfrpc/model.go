package msfrpc

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/vmihailenco/msgpack/v5/msgpcode"
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
	case msgpcode.Uint16:
		val, err := decoder.DecodeUint16()
		if err != nil {
			return err
		}
		*e = errorCode(val)
	case msgpcode.Bin8:
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
		return errors.Errorf("unknown code about error code: %x", code)
	}
	return nil
}

// MSFError is an error about Metasploit RPC.
type MSFError struct {
	Err            bool      `msgpack:"error"           json:"error"`
	ErrorClass     string    `msgpack:"error_class"     json:"error_class"`
	ErrorString    string    `msgpack:"error_string"    json:"error_string"`
	ErrorBacktrace []string  `msgpack:"error_backtrace" json:"error_backtrace"`
	ErrorMessage   string    `msgpack:"error_message"   json:"error_message"`
	ErrorCode      errorCode `msgpack:"error_code"      json:"error_code"`
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
	ErrInvalidJobID           = "Invalid Job"
	ErrInvalidJobIDPrefix     = "invalid job id: "
	ErrInvalidModule          = "Invalid Module"
	ErrInvalidModuleFormat    = "invalid module: %s/%s"
	ErrUnknownSessionID       = "Unknown Session ID "
	ErrUnknownSessionIDPrefix = "unknown session id: "
)

const (
	// structTag is used to xreflect.StructureToMap().
	structTag = "msgpack"

	// defaultWorkspace is the default workspace name.
	defaultWorkspace = "default"
)

// about executable file template
const (
	TemplateX86WindowsEXE = "template_x86_windows.exe"
	TemplateX64WindowsEXE = "template_x64_windows.exe"
	TemplateX86LinuxELF   = "template_x86_linux.bin"
	TemplateX64LinuxELF   = "template_x64_linux.bin"
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
	MethodDBReportClient     = "db.report_client"
	MethodDBClients          = "db.clients"
	MethodDBGetClient        = "db.get_client"
	MethodDBDelClient        = "db.del_client"
	MethodDBCreateCred       = "db.create_credential"
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
	Method      string `json:"-"`
	Token       string `json:"-"`
	LogoutToken string `json:"token"` // will be deleted
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
	Method   string `json:"-"`
	Token    string `json:"-"`
	NewToken string `json:"token"`
}

// AuthTokenAddResult is the result about add token.
type AuthTokenAddResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// AuthTokenRemoveRequest is used to remove token.
type AuthTokenRemoveRequest struct {
	Method           string `json:"-"`
	Token            string `json:"-"`
	TokenToBeRemoved string `json:"token"`
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
	Exploit   int `msgpack:"exploits"  json:"exploits"`
	Auxiliary int `msgpack:"auxiliary" json:"auxiliary"`
	Post      int `msgpack:"post"      json:"post"`
	Payload   int `msgpack:"payloads"  json:"payloads"`
	Encoder   int `msgpack:"encoders"  json:"encoders"`
	Nop       int `msgpack:"nops"      json:"nops"`
	MSFError
}

// CoreAddModulePathRequest is used to add module.
type CoreAddModulePathRequest struct {
	Method string `json:"-"`
	Token  string `json:"-"`
	Path   string `json:"path"`
}

// CoreAddModulePathResult is the result about add module.
type CoreAddModulePathResult struct {
	Exploit   int `msgpack:"exploits"  json:"exploits"`
	Auxiliary int `msgpack:"auxiliary" json:"auxiliary"`
	Post      int `msgpack:"post"      json:"post"`
	Payload   int `msgpack:"payloads"  json:"payloads"`
	Encoder   int `msgpack:"encoders"  json:"encoders"`
	Nop       int `msgpack:"nops"      json:"nops"`
	MSFError
}

// CoreReloadModulesRequest is used to reload modules.
type CoreReloadModulesRequest struct {
	Method string
	Token  string
}

// CoreReloadModulesResult is the result about reload modules.
type CoreReloadModulesResult struct {
	Exploit   int `msgpack:"exploits"  json:"exploits"`
	Auxiliary int `msgpack:"auxiliary" json:"auxiliary"`
	Post      int `msgpack:"post"      json:"post"`
	Payload   int `msgpack:"payloads"  json:"payloads"`
	Encoder   int `msgpack:"encoders"  json:"encoders"`
	Nop       int `msgpack:"nops"      json:"nops"`
	MSFError
}

// CoreThreadListRequest is used to get thread list.
type CoreThreadListRequest struct {
	Method string
	Token  string
}

// CoreThreadInfo contains the information about thread.
type CoreThreadInfo struct {
	Status   string `msgpack:"status"   json:"status"`
	Critical bool   `msgpack:"critical" json:"critical"`
	Name     string `msgpack:"name"     json:"name"`
	Started  string `msgpack:"started"  json:"started"`
}

// CoreThreadKillRequest is used to kill thread by ID.
type CoreThreadKillRequest struct {
	Method string `json:"-"`
	Token  string `json:"-"`
	ID     uint64 `json:"id"`
}

// CoreThreadKillResult is the result about kill thread.
type CoreThreadKillResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// CoreSetGRequest is used to set global option.
type CoreSetGRequest struct {
	Method string `json:"-"`
	Token  string `json:"-"`
	Name   string `json:"name"`
	Value  string `json:"value"`
}

// CoreSetGResult is the result of set global option.
type CoreSetGResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// CoreUnsetGRequest is used to unset global option.
type CoreUnsetGRequest struct {
	Method string `json:"-"`
	Token  string `json:"-"`
	Name   string `json:"name"`
}

// CoreUnsetGResult is the result of unset global option.
type CoreUnsetGResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// CoreGetGRequest is used to get global option.
type CoreGetGRequest struct {
	Method string `json:"-"`
	Token  string `json:"-"`
	Name   string `json:"name"`
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
	Version string `msgpack:"version" json:"version"`
	Ruby    string `msgpack:"ruby"    json:"ruby"`
	API     string `msgpack:"api"     json:"api"`
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
	Driver   string                 `toml:"driver"`
	Host     string                 `toml:"host"`
	Port     uint64                 `toml:"port"`
	Username string                 `toml:"username"`
	Password string                 `toml:"password"`
	Database string                 `toml:"database"`
	Options  map[string]interface{} `toml:"options"`
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
	for key, value := range opts.Options {
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
	Driver   string `msgpack:"driver" json:"driver"`
	Database string `msgpack:"db"     json:"database"`
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
	Workspace     string `msgpack:"workspace"    json:"workspace"`
	Name          string `msgpack:"name"         json:"name"`
	Host          string `msgpack:"host"         json:"host"`
	MAC           string `msgpack:"mac"          json:"mac"`
	OSName        string `msgpack:"os_name"      json:"os_name"`
	OSFlavor      string `msgpack:"os_flavor"    json:"os_flavor"`
	OSServicePack string `msgpack:"os_sp"        json:"os_service_pack"`
	OSLanguage    string `msgpack:"os_lang"      json:"os_language"`
	Architecture  string `msgpack:"arch"         json:"architecture"`
	State         string `msgpack:"state"        json:"state"`
	Scope         string `msgpack:"scope"        json:"scope"`
	VirtualHost   string `msgpack:"virtual_host" json:"virtual_host"`
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
	Name          string `msgpack:"name"       json:"name"`
	Address       string `msgpack:"address"    json:"address"`
	MAC           string `msgpack:"mac"        json:"mac"`
	OSName        string `msgpack:"os_name"    json:"os_name"`
	OSFlavor      string `msgpack:"os_flavor"  json:"os_flavor"`
	OSServicePack string `msgpack:"os_sp"      json:"os_service_pack"`
	OSLanguage    string `msgpack:"os_lang"    json:"os_language"`
	Purpose       string `msgpack:"purpose"    json:"purpose"`
	Information   string `msgpack:"info"       json:"information"`
	State         string `msgpack:"state"      json:"state"`
	CreatedAt     int64  `msgpack:"created_at" json:"created_at"`
	UpdateAt      int64  `msgpack:"updated_at" json:"updated_at"`
}

// DBGetHostRequest is used to get host information.
type DBGetHostRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBGetHostOptions contains options about get host.
type DBGetHostOptions struct {
	Workspace string `msgpack:"workspace" json:"workspace"`
	Address   string `msgpack:"address"   json:"address"`
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

// DBDelHostOptions contains options about delete host.
type DBDelHostOptions struct {
	Workspace string `msgpack:"workspace" json:"workspace"`
	Address   string `msgpack:"address"   json:"address"`
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
	Workspace string `msgpack:"workspace" json:"workspace"`
	Host      string `msgpack:"host"      json:"host"`
	Port      string `msgpack:"port"      json:"port"`
	Protocol  string `msgpack:"proto"     json:"protocol"`
	Name      string `msgpack:"name"      json:"name"`
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
	Workspace string `msgpack:"workspace" json:"workspace"`
	Limit     uint64 `msgpack:"limit"     json:"limit"`
	Offset    uint64 `msgpack:"offset"    json:"offset"`
	Address   string `msgpack:"address"   json:"address"`
	Port      string `msgpack:"port"      json:"port"`
	Protocol  string `msgpack:"proto"     json:"protocol"`
	Name      string `msgpack:"name"      json:"name"`
}

// DBServicesResult is the result of get services by filter.
type DBServicesResult struct {
	Services []*DBService `msgpack:"services"`
	MSFError
}

// DBService contains server information.
type DBService struct {
	Host        string `msgpack:"host"       json:"host"`
	Port        uint64 `msgpack:"port"       json:"port"`
	Protocol    string `msgpack:"proto"      json:"protocol"`
	Name        string `msgpack:"name"       json:"name"`
	State       string `msgpack:"state"      json:"state"`
	Information string `msgpack:"info"       json:"information"`
	CreatedAt   int64  `msgpack:"created_at" json:"created_at"`
	UpdateAt    int64  `msgpack:"updated_at" json:"updated_at"`
}

// DBGetServiceRequest is used to get service.
type DBGetServiceRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBGetServiceOptions contains options about get service.
type DBGetServiceOptions struct {
	Workspace string `msgpack:"workspace" json:"workspace"`
	Protocol  string `msgpack:"proto"     json:"protocol"`
	Port      uint64 `msgpack:"port"      json:"port"`
	Names     string `msgpack:"names"     json:"names"`
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
	Workspace string   `msgpack:"workspace" json:"workspace"`
	Address   string   `msgpack:"address"   json:"address"`
	Addresses []string `msgpack:"addresses" json:"addresses"`
	Port      uint64   `msgpack:"port"      json:"port"`
	Protocol  string   `msgpack:"proto"     json:"protocol"`
}

// DBDelServiceResult is the result of delete service.
type DBDelServiceResult struct {
	Result  string          `msgpack:"result"`
	Deleted []*DBDelService `msgpack:"deleted"`
	MSFError
}

// DBDelService contains information about deleted service.
type DBDelService struct {
	Address  string `msgpack:"address" json:"address"`
	Port     uint64 `msgpack:"port"    json:"port"`
	Protocol string `msgpack:"proto"   json:"protocol"`
}

// DBReportClientRequest is used to add browser client to database.
type DBReportClientRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBReportClient contains information about report browser client.
type DBReportClient struct {
	Workspace string `msgpack:"workspace" json:"workspace"`
	Host      string `msgpack:"host"      json:"host"`
	UAString  string `msgpack:"ua_string" json:"ua_string"`
	UAName    string `msgpack:"ua_name"   json:"ua_name"`
	UAVersion string `msgpack:"ua_ver"    json:"ua_version"`
}

// DBReportClientResult is the result of add browser client to database.
type DBReportClientResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// DBClientsRequest is used to get browser clients by filter.
type DBClientsRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBClientsOptions contains options about get browser clients by filter.
type DBClientsOptions struct {
	Workspace string   `msgpack:"workspace" json:"workspace"`
	Addresses []string `msgpack:"addresses" json:"addresses"`
	UAName    string   `msgpack:"ua_name"   json:"ua_name"`
	UAVersion string   `msgpack:"ua_ver"    json:"ua_version"`
}

// DBClientsResult is the result of get browser clients by filter.
type DBClientsResult struct {
	Clients []*DBClient `msgpack:"clients"`
	MSFError
}

// DBClient contains information about browser client.
type DBClient struct {
	Host      string `msgpack:"host"       json:"host"`
	UAString  string `msgpack:"ua_string"  json:"ua_string"`
	UAName    string `msgpack:"ua_name"    json:"ua_name"`
	UAVersion string `msgpack:"ua_ver"     json:"ua_version"`
	CreatedAt int64  `msgpack:"created_at" json:"created_at"`
	UpdateAt  int64  `msgpack:"updated_at" json:"updated_at"`
}

// DBGetClientRequest is used of get browser client by filter.
type DBGetClientRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBGetClientOptions contain options about get browser client by filter.
type DBGetClientOptions struct {
	Workspace string `msgpack:"workspace" json:"workspace"`
	Host      string `msgpack:"host"      json:"host"`
	UAString  string `msgpack:"ua_string" json:"ua_string"`
}

// DBGetClientResult is the result of get browser client by filter.
type DBGetClientResult struct {
	Client []*DBClient `msgpack:"client"`
	MSFError
}

// DBDelClientRequest is used to delete browser client.
type DBDelClientRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBDelClientOptions contains options about delete browser client.
type DBDelClientOptions struct {
	Workspace string   `msgpack:"workspace" json:"workspace"`
	Address   string   `msgpack:"address"   json:"address"`
	Addresses []string `msgpack:"addresses" json:"addresses"`
	UAName    string   `msgpack:"ua_name"   json:"ua_name"`
	UAVersion string   `msgpack:"ua_ver"    json:"ua_version"`
}

// DBDelClientResult is the result of delete browser client.
type DBDelClientResult struct {
	Result  string         `msgpack:"result"`
	Deleted []*DBDelClient `msgpack:"deleted"`
	MSFError
}

// DBDelClient contains information about deleted browser client.
type DBDelClient struct {
	Address  string `msgpack:"address"   json:"address"`
	UAString string `msgpack:"ua_string" json:"ua_string"`
}

// DBCreateCredentialRequest is used to create credential.
type DBCreateCredentialRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBCreateCredentialOptions contains options about create credential.
type DBCreateCredentialOptions struct {
	OriginType     string `msgpack:"origin_type"     json:"origin_type"`
	ServiceName    string `msgpack:"service_name"    json:"service_name"`
	Address        string `msgpack:"address"         json:"address"`
	Port           uint64 `msgpack:"port"            json:"port"`
	Protocol       string `msgpack:"protocol"        json:"protocol"`
	ModuleFullname string `msgpack:"module_fullname" json:"module_fullname"`
	Username       string `msgpack:"username"        json:"username"`
	PrivateType    string `msgpack:"private_type"    json:"private_type"`
	PrivateData    string `msgpack:"private_data"    json:"private_data"`
	WorkspaceID    uint64 `msgpack:"workspace_id"    json:"workspace_id"`
}

// DBCreateCredentialResult is the result of create credential.
type DBCreateCredentialResult struct {
	Host        string `msgpack:"host"         json:"host"`
	Username    string `msgpack:"username"     json:"username"`
	PrivateType string `msgpack:"private_type" json:"private_type"`
	Private     string `msgpack:"private"      json:"private"`
	RealmValue  string `msgpack:"realm_value"  json:"realm_value"`
	RealmKey    string `msgpack:"realm_key"    json:"realm_key"`
	ServiceName string `msgpack:"sname"        json:"service_name"`
	Status      string `msgpack:"status"       json:"status"`
	MSFError
}

// DBCredsRequest is used to get credentials.
type DBCredsRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBCredsResult is the result of get credentials.
type DBCredsResult struct {
	Credentials []*DBCred `msgpack:"creds"`
	MSFError
}

// DBCred contains information about credential.
type DBCred struct {
	Host        string `msgpack:"host"       json:"host"`
	Port        uint64 `msgpack:"port"       json:"port"`
	Protocol    string `msgpack:"proto"      json:"protocol"`
	ServiceName string `msgpack:"sname"      json:"service_name"`
	Type        string `msgpack:"type"       json:"type"`
	Username    string `msgpack:"user"       json:"username"`
	Password    string `msgpack:"pass"       json:"password"`
	UpdateAt    int64  `msgpack:"updated_at" json:"updated_at"`
}

// DBDelCredsRequest is used to delete credential.
type DBDelCredsRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBDelCredsResult is the result of delete credential.
type DBDelCredsResult struct {
	Result  string `msgpack:"result"`
	Deleted []struct {
		Creds []*DBDelCred `msgpack:"creds"`
	} `msgpack:"deleted"`
	MSFError
}

// DBDelCred contains the information of deleted credential.
type DBDelCred struct {
	Host        string `msgpack:"host"       json:"host"`
	Port        uint64 `msgpack:"port"       json:"port"`
	Protocol    string `msgpack:"proto"      json:"protocol"`
	ServiceName string `msgpack:"sname"      json:"service_name"`
	Type        string `msgpack:"type"       json:"type"`
	Username    string `msgpack:"user"       json:"username"`
	Password    string `msgpack:"pass"       json:"password"`
	UpdateAt    int64  `msgpack:"updated_at" json:"updated_at"`
}

// DBReportLootRequest is used to add a loot to database.
type DBReportLootRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBReportLoot contains information about loot.
type DBReportLoot struct {
	Workspace   string `msgpack:"workspace"    json:"workspace"`
	Host        string `msgpack:"host"         json:"host"`
	Port        uint64 `msgpack:"port"         json:"port"`
	Proto       string `msgpack:"proto"        json:"protocol"`
	Name        string `msgpack:"name"         json:"name"`
	Type        string `msgpack:"type"         json:"type"`
	ContentType string `msgpack:"content_type" json:"content_type"`
	Path        string `msgpack:"path"         json:"path"`
	Data        string `msgpack:"data"         json:"data"`
	Information string `msgpack:"info"         json:"information"`
}

// DBReportLootResult is the result of add loot.
type DBReportLootResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// DBLootsRequest is used to get loots.
type DBLootsRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBLootsOptions contains options about loots.
type DBLootsOptions struct {
	Workspace string `msgpack:"workspace" json:"workspace"`
	Limit     uint64 `msgpack:"limit"     json:"limit"`
	Offset    uint64 `msgpack:"offset"    json:"offset"`
}

// DBLootsResult is the result of get loots.
type DBLootsResult struct {
	Loots []*DBLoot `msgpack:"loots"`
	MSFError
}

// DBLoot contains information about loot.
type DBLoot struct {
	Host        string                 `msgpack:"host"       json:"host"`
	Service     string                 `msgpack:"service"    json:"service"`
	Name        string                 `msgpack:"name"       json:"name"`
	LootType    string                 `msgpack:"ltype"      json:"loot_type"`
	ContentType string                 `msgpack:"ctype"      json:"content_type"`
	Data        map[string]interface{} `msgpack:"data"       json:"data"`
	Information string                 `msgpack:"info"       json:"information"`
	CreatedAt   int64                  `msgpack:"created_at" json:"created_at"`
	UpdateAt    int64                  `msgpack:"updated_at" json:"updated_at"`
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
	ID        uint64 `msgpack:"id"         json:"id"`
	Name      string `msgpack:"name"       json:"name"`
	CreatedAt int64  `msgpack:"created_at" json:"created_at"`
	UpdateAt  int64  `msgpack:"updated_at" json:"updated_at"`
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

// DBSetWorkspaceRequest is used to set the current workspace.
type DBSetWorkspaceRequest struct {
	Method string
	Token  string
	Name   string
}

// DBSetWorkspaceResult is the result of set the current workspace.
type DBSetWorkspaceResult struct {
	Result string `msgpack:"result"`
	MSFError
}

// DBCurrentWorkspaceRequest is used to get the current workspace.
type DBCurrentWorkspaceRequest struct {
	Method string
	Token  string
}

// DBCurrentWorkspaceResult is the result of get the current workspace.
type DBCurrentWorkspaceResult struct {
	Name string `msgpack:"workspace"    json:"workspace"`
	ID   uint64 `msgpack:"workspace_id" json:"workspace_id"`
	MSFError
}

// DBEventRequest is used to get framework events.
type DBEventRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBEventOptions contains options about get events.
type DBEventOptions struct {
	Workspace string `msgpack:"workspace" json:"workspace"`
	Limit     uint64 `msgpack:"limit"     json:"limit"`
	Offset    uint64 `msgpack:"offset"    json:"offset"`
}

// DBEventResult is the result of get framework events.
type DBEventResult struct {
	Events []*DBEvent `msgpack:"events"`
	MSFError
}

// DBEvent contains information about framework event.
type DBEvent struct {
	Name        string                 `msgpack:"name"       json:"name"`
	Critical    bool                   `msgpack:"critical"   json:"critical"`
	Host        string                 `msgpack:"host"       json:"host"`
	Username    string                 `msgpack:"username"   json:"username"`
	Information map[string]interface{} `msgpack:"info"       json:"information"`
	CreatedAt   int64                  `msgpack:"created_at" json:"created_at"`
	UpdateAt    int64                  `msgpack:"updated_at" json:"updated_at"`
}

// DBImportDataRequest is used to import external data to database.
type DBImportDataRequest struct {
	Method  string
	Token   string
	Options map[string]interface{}
}

// DBImportDataOptions contains options about import data.
type DBImportDataOptions struct {
	Workspace string `msgpack:"workspace" json:"workspace"`
	Data      string `msgpack:"data"      json:"data"`
}

// DBImportDataResult is the result of import external data.
type DBImportDataResult struct {
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
	ID     string `msgpack:"id"     json:"id"`
	Prompt string `msgpack:"prompt" json:"prompt"`
	Busy   bool   `msgpack:"busy"   json:"busy"`
}

// ConsoleCreateRequest is used to create a console.
type ConsoleCreateRequest struct {
	Method  string
	Token   string
	Options map[string]string
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

type license []string

func (license *license) DecodeMsgpack(decoder *msgpack.Decoder) error {
	code, err := decoder.PeekCode()
	if err != nil {
		return err
	}
	switch {
	case msgpcode.IsBin(code):
		str, err := decoder.DecodeString()
		if err != nil {
			return err
		}
		*license = []string{str}
	case msgpcode.IsFixedArray(code):
		slice, err := decoder.DecodeSlice()
		if err != nil {
			return err
		}
		ll := len(slice)
		ls := make([]string, ll)
		for i := 0; i < ll; i++ {
			ls[i] = string(slice[i].([]byte))
		}
		*license = ls
	default:
		return errors.Errorf("unknown code about license: %x", code)
	}
	return nil
}

// ModuleInfoResult is the result about get module's information.
type ModuleInfoResult struct {
	Type           string                   `msgpack:"type"           json:"type"`
	Name           string                   `msgpack:"name"           json:"name"`
	FullName       string                   `msgpack:"fullname"       json:"fullname"`
	Rank           string                   `msgpack:"rank"           json:"rank"`
	DisclosureDate string                   `msgpack:"disclosuredate" json:"disclosure_date"`
	Description    string                   `msgpack:"description"    json:"description"`
	License        license                  `msgpack:"license"        json:"license"`
	Filepath       string                   `msgpack:"filepath"       json:"filepath"`
	Arch           []string                 `msgpack:"arch"           json:"architecture"`
	Platform       []string                 `msgpack:"platform"       json:"platform"`
	Targets        map[uint64]string        `msgpack:"targets"        json:"targets"`
	DefaultTarget  uint64                   `msgpack:"default_target" json:"default_target"`
	Actions        map[uint64]string        `msgpack:"actions"        json:"actions"`
	DefaultAction  string                   `msgpack:"default_action" json:"default_action"`
	Privileged     bool                     `msgpack:"privileged"     json:"privileged"`
	Authors        []string                 `msgpack:"authors"        json:"authors"`
	References     []interface{}            `msgpack:"references"     json:"references"`
	Stance         string                   `msgpack:"stance"         json:"stance"`
	Options        map[string]*ModuleOption `msgpack:"options"        json:"options"`
	MSFError
}

// ModuleOption contains modules information about options.
type ModuleOption struct {
	Type        string        `msgpack:"type"     json:"type"`
	Required    bool          `msgpack:"required" json:"required"`
	Advanced    bool          `msgpack:"advanced" json:"advanced"`
	Description string        `msgpack:"desc"     json:"description"`
	Enums       []interface{} `msgpack:"enums"    json:"enumerations"`
	Default     interface{}   `msgpack:"default"  json:"default"`
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
	Type        string        `msgpack:"type"     json:"type"`
	Required    bool          `msgpack:"required" json:"required"`
	Advanced    bool          `msgpack:"advanced" json:"advanced"`
	Evasion     bool          `msgpack:"evasion"  json:"evasion"`
	Description string        `msgpack:"desc"     json:"description"`
	Enums       []interface{} `msgpack:"enums"    json:"enumerations"`
	Default     interface{}   `msgpack:"default"  json:"default"`
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
	Format       string `json:"format"`
	BadChars     string `json:"bad_chars"`
	Platform     string `json:"platform"`
	Arch         string `json:"architecture"`
	EncodeCount  uint64 `json:"encode_count"`
	Inject       bool   `json:"inject"`
	AltEXE       string `json:"alt_exe"`
	EXEDir       string `json:"exe_dir"`
	AddShellcode string `json:"add_shellcode"`
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
	BadChars            string                 `json:"bad_chars"`
	Format              string                 `json:"format"`
	ForceEncoding       bool                   `json:"force_encoding"`
	Template            string                 `json:"template"`
	Platform            string                 `json:"platform"`
	KeepTemplateWorking bool                   `json:"keep_template_working"`
	NopSledSize         uint64                 `json:"nop_sled_size"`
	Iterations          uint64                 `json:"iterations"`
	DataStore           map[string]interface{} `json:"data_store"`
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
	JobID   uint64 `msgpack:"job_id"  json:"job_id"`
	UUID    string `msgpack:"uuid"    json:"uuid"`
	Payload string `msgpack:"payload" json:"payload"`
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
	JobID uint64 `msgpack:"job_id" json:"job_id"`
	UUID  string `msgpack:"uuid"   json:"uuid"`
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
	Waiting []string `msgpack:"waiting" json:"waiting"`
	Running []string `msgpack:"running" json:"running"`
	Results []string `msgpack:"results" json:"results"`
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
	JobID     uint64                 `msgpack:"jid"        json:"job_id"`
	Name      string                 `msgpack:"name"       json:"name"`
	StartTime uint64                 `msgpack:"start_time" json:"start_time"`
	DataStore map[string]interface{} `msgpack:"datastore"  json:"data_store"`
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
	Type         string `msgpack:"type"         json:"type"`
	ViaExploit   string `msgpack:"via_exploit"  json:"via_exploit"`
	ViaPayload   string `msgpack:"via_payload"  json:"via_payload"`
	TunnelLocal  string `msgpack:"tunnel_local" json:"tunnel_local"`
	TunnelPeer   string `msgpack:"tunnel_peer"  json:"tunnel_peer"`
	SessionHost  string `msgpack:"session_host" json:"session_host"`
	SessionPort  uint64 `msgpack:"session_port" json:"session_port"`
	TargetHost   string `msgpack:"target_host"  json:"target_host"`
	Routes       string `msgpack:"routes"       json:"routes"`
	Username     string `msgpack:"username"     json:"username"`
	Architecture string `msgpack:"arch"         json:"architecture"`
	Platform     string `msgpack:"platform"     json:"platform"`
	Description  string `msgpack:"desc"         json:"description"`
	Information  string `msgpack:"info"         json:"information"`
	Workspace    string `msgpack:"workspace"    json:"workspace"`
	UUID         string `msgpack:"uuid"         json:"uuid"`
	ExploitUUID  string `msgpack:"exploit_uuid" json:"exploit_uuid"`
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
	Method string
	Token  string
	ID     uint64
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
