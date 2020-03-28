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

const success = "success"

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
	MethodCoreGetG          = "core.getg"
	MethodCoreSetG          = "core.setg"
	MethodCoreUnsetG        = "core.unsetg"
	MethodCoreSave          = "core.save"
	MethodCoreVersion       = "core.version"

	// MethodConsoleCreate  = "console.create"
	// MethodConsoleList    = "console.list"
	// MethodConsoleTabs    = "console.tabs"
	// MethodConsoleDestroy = "console.destroy"
	// MethodConsoleRead    = "console.read"
	// MethodConsoleWrite   = "console.write"

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
