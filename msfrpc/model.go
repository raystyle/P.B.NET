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

	MethodConsoleCreate        = "console.create"
	MethodConsoleDestroy       = "console.destroy"
	MethodConsoleRead          = "console.read"
	MethodConsoleWrite         = "console.write"
	MethodConsoleList          = "console.list"
	MethodConsoleSessionDetach = "console.session_detach"
	MethodConsoleSessionKill   = "console.session_kill"

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

	MethodPluginLoad   = "plugin.load"
	MethodPluginUnload = "plugin.unload"
	MethodPluginLoaded = "plugin.loaded"

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

	MethodJobList = "job.list"
	MethodJobInfo = "job.info"
	MethodJobStop = "job.stop"

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
