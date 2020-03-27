package msfrpc

// MSFError is an error about Metasploit RPC.
type MSFError struct {
	Err            bool     `msgpack:"error"`
	ErrorClass     string   `msgpack:"error_class"`
	ErrorString    string   `msgpack:"error_string"`
	ErrorBacktrace []string `msgpack:"error_backtrace"`
	ErrorMessage   string   `msgpack:"error_message"`
	ErrorCode      int64    `msgpack:"error_code"`
}

func (err *MSFError) Error() string {
	return err.ErrorMessage
}

// for msgpack marshal as array.
type asArray interface {
	asArray()
}

const success = "success"

// ------------------------------------------about methods-----------------------------------------
const (
	MethodAuthLogin         = "auth.login"
	MethodAuthLogout        = "auth.logout"
	MethodAuthTokenList     = "auth.token_list"     // #nosec
	MethodAuthTokenGenerate = "auth.token_generate" // #nosec
	MethodAuthTokenAdd      = "auth.token_add"      // #nosec
	MethodAuthTokenRemove   = "auth.token_remove"   // #nosec

// MethodConsoleCreate  = "console.create"
// MethodConsoleList    = "console.list"
// MethodConsoleTabs    = "console.tabs"
// MethodConsoleDestroy = "console.destroy"
// MethodConsoleRead    = "console.read"
// MethodConsoleWrite   = "console.write"
//
// MethodCoreVersion       = "core.version"
// MethodCoreSetG          = "core.setg"
// MethodCoreUnsetG        = "core.unsetg"
// MethodCoreModuleStats   = "core.module_stats"
// MethodCoreReloadModules = "core.reload_modules"
// MethodCoreAddModulePath = "core.add_module_path"
// MethodCoreThreadList    = "core.thread_list"
// MethodCoreThreadKill    = "core.thread_kill"
// MethodCoreSave          = "core.save"
// MethodCoreStop          = "core.stop"
//
// MethodDBConnect          = "db.connect"
// MethodDBStatus           = "db.status"
// MethodDBDisconnect       = "db.disconnect"
// MethodDBHosts            = "db.hosts"
// MethodDBServices         = "db.services"
// MethodDBVulns            = "db.vulns"
// MethodDBWorkspaces       = "db.workspaces"
// MethodDBCurrentWorkspace = "db.current_workspace"
// MethodDBGetWorkspace     = "db.get_workspace"
// MethodDBSetWorkspace     = "db.set_workspace"
// MethodDBDelWorkspace     = "db.del_workspace"
// MethodDBAddWorkspace     = "db.add_workspace"
// MethodDBGetHost          = "db.get_host"
// MethodDBReportHost       = "db.report_host"
// MethodDBReportService    = "db.report_service"
// MethodDBGetService       = "db.get_service"
// MethodDBGetNote          = "db.get_note"
// MethodDBGetClient        = "db.get_client"
// MethodDBReportClient     = "db.report_client"
// MethodDBReportNote       = "db.report_note"
// MethodDBNotes            = "db.notes"
// MethodDBReportAuthInfo   = "db.report_auth_info"
// MethodDBGetAuthInfo      = "db.get_auth_info"
// MethodDBGetRef           = "db.get_ref"
// MethodDBDelVuln          = "db.del_vuln"
// MethodDBDelNote          = "db.del_note"
// MethodDBDelService       = "db.del_service"
// MethodDBDelHost          = "db.del_host"
// MethodDBReportVuln       = "db.report_vuln"
// MethodDBEvents           = "db.events"
// MethodDBReportEvent      = "db.report_event"
// MethodDBReportLoot       = "db.report_loot"
// MethodDBLoots            = "db.loots"
// MethodDBReportCred       = "db.report_cred"
// MethodDBCreds            = "db.creds"
// MethodDBImportData       = "db.import_data"
// MethodDBGetVuln          = "db.get_vuln"
// MethodDBClients          = "db.clients"
// MethodDBDelClient        = "db.del_client"
// MethodDBDriver           = "db.driver"
//
// MethodPluginLoad   = "plugin.load"
// MethodPluginUnload = "plugin.unload"
// MethodPluginLoaded = "plugin.loaded"
//
// MethodModuleExploits                 = "module.exploits"
// MethodModuleAuxiliary                = "module.auxiliary"
// MethodModulePayloads                 = "module.payloads"
// MethodModuleEncoders                 = "module.encoders"
// MethodModuleNops                     = "module.nops"
// MethodModulePost                     = "module.post"
// MethodModuleInfo                     = "module.info"
// MethodModuleCompatiblePayloads       = "module.compatible_payloads"
// MethodModuleCompatibleSessions       = "module.compatible_sessions"
// MethodModuleTargetCompatiblePayloads = "module.target_compatible_payloads"
// MethodModuleOptions                  = "module.options"
// MethodModuleExecute                  = "module.execute"
// MethodModuleEncodeFormats            = "module.encode_formats"
// MethodModuleEncode                   = "module.encode"
//
// MethodJobList = "job.list"
// MethodJobStop = "job.stop"
// MethodJobInfo = "job.info"
//
// MethodSessionList                          = "session.list"
// MethodSessionStop                          = "session.stop"
// MethodSessionShellRead                     = "session.shell_read"
// MethodSessionShellWrite                    = "session.shell_write"
// MethodSessionShellUpgrade                  = "session.shell_upgrade"
// MethodSessionRingRead                      = "session.ring_read"
// MethodSessionRingPut                       = "session.ring_put"
// MethodSessionRingLast                      = "session.ring_last"
// MethodSessionRingClear                     = "session.ring_clear"
// MethodSessionMeterpreterRead               = "session.meterpreter_read"
// MethodSessionMeterpreterWrite              = "session.meterpreter_write"
// MethodSessionMeterpreterSessionDetach      = "session.meterpreter_session_detach"
// MethodSessionMeterpreterSessionKill        = "session.meterpreter_session_kill"
// MethodSessionMeterpreterTabs               = "session.meterpreter_tabs"
// MethodSessionMeterpreterRunSingle          = "session.meterpreter_run_single"
// MethodSessionMeterpreterScript             = "session.meterpreter_script"
// MethodSessionMeterpreterDirectorySeparator = "session.meterpreter_directory_separator"
// MethodSessionCompatibleModules             = "session.compatible_modules"
)

// -------------------------------------------about auth-------------------------------------------

// AuthLoginRequest is used to login and get token.
type AuthLoginRequest struct {
	Method   string
	Username string
	Password string
}

func (alr *AuthLoginRequest) asArray() {}

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

func (alr *AuthLogoutRequest) asArray() {}

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

func (atr *AuthTokenListRequest) asArray() {}

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

func (ata *AuthTokenAddRequest) asArray() {}

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

func (atr *AuthTokenRemoveRequest) asArray() {}

// AuthTokenRemoveResult is the result about remove token.
type AuthTokenRemoveResult struct {
	Result string `msgpack:"result"`
	MSFError
}
