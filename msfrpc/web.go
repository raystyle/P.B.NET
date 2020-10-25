package msfrpc

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/netutil"

	"project/internal/httptool"
	"project/internal/logger"
	"project/internal/option"
	"project/internal/patch/json"
	"project/internal/random"
	"project/internal/security"
	"project/internal/virtualconn"
	"project/internal/xpanic"
	"project/internal/xreflect"
)

const (
	defaultAdminUsername    = "admin"
	minRequestBodySize      = 4 * 1024 * 1024  // 4MB
	minRequestLargeBodySize = 64 * 1024 * 1024 // 64MB
)

// WebOptions contains options about web server.
type WebOptions struct {
	// AdminUsername is the administrator username,
	// if it is empty, use the default admin username
	AdminUsername string `toml:"admin_username"`

	// AdminPassword is the administrator hashed password(bcrypt),
	// if it is empty, program will generate a random value
	AdminPassword string `toml:"admin_password"`

	// MaxConns is the web server maximum connections
	MaxConns int `toml:"max_conns"`

	// MaxBodySize is the incoming request maximum body size
	MaxBodySize int64 `toml:"max_body_size"`

	// MaxLargeBodySize is the incoming large request maximum
	// body size, like upload a file, or some else.
	MaxLargeBodySize int64 `toml:"max_large_body_size"`

	// HFS is used to use custom file system
	HFS http.FileSystem `toml:"-" msgpack:"-"`

	// APIOnly is used to disable Web UI
	APIOnly bool `toml:"api_only"`

	// Server contains options about http server.
	Server option.HTTPServer `toml:"server" check:"-"`

	// Users contains common users, key is the username and
	// value is the hashed password(bcrypt)
	Users map[string]string `toml:"users"`
}

// Web is provide a web UI and API server.
type Web struct {
	maxConns int
	server   *http.Server
	ui       *webUI
	api      *webAPI
}

// NewWeb is used to create a web server, password is the common user password.
func NewWeb(client *Client, opts *WebOptions) (*Web, error) {
	server, err := opts.Server.Apply()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	mux := http.NewServeMux()

	// web := &Web{}

	if !opts.APIOnly {
		webUI, err := newWebUI(opts.HFS, mux)
		if err != nil {
			return nil, err
		}
		fmt.Println(webUI)
	}

	// set web server
	server.Handler = mux
	server.ConnState = func(conn net.Conn, state http.ConnState) {

	}

	server.ErrorLog = logger.Wrap(logger.Warning, "msfrpc-web", client.logger)
	web := Web{
		server: server,
	}
	if web.maxConns < 32 {
		web.maxConns = 1000
	}
	return &web, nil
}

// MonitorCallbacks is used to return callbacks for monitor.
func (web *Web) MonitorCallbacks() *MonitorCallbacks {
	return &MonitorCallbacks{
		OnToken:      web.api.onToken,
		OnJob:        web.api.onJob,
		OnSession:    web.api.onSession,
		OnHost:       web.api.onHost,
		OnCredential: web.api.onCredential,
		OnLoot:       web.api.onLoot,
		OnEvent:      web.api.onEvent,
	}
}

// Serve is used to start web server.
func (web *Web) Serve(listener net.Listener) error {
	l := netutil.LimitListener(listener, web.maxConns)
	switch listener.(type) {
	case *virtualconn.Listener:
		return web.server.Serve(l)
	default:
		return web.server.ServeTLS(l, "", "")
	}
}

// Close is used to close web server.
func (web *Web) Close() error {
	return web.server.Close()
}

// webUI is used to contains favicon and index data. user can reload it.
type webUI struct {
	hfs     http.FileSystem
	favicon []byte
	index   []byte
	rwm     sync.RWMutex
}

func newWebUI(hfs http.FileSystem, mux *http.ServeMux) (*webUI, error) {
	ui := webUI{hfs: hfs}
	err := ui.Reload()
	if err != nil {
		return nil, err
	}
	mux.HandleFunc("/favicon.ico", ui.handleFavicon)
	// set index handler
	for _, name := range [...]string{
		"", "index.html", "index.htm", "index",
	} {
		mux.HandleFunc("/"+name, ui.handleIndex)
	}
	// set resource server
	for _, path := range [...]string{
		"css", "js", "img", "fonts",
	} {
		mux.Handle("/"+path+"/", http.FileServer(hfs))
	}
	return &ui, nil
}

func (ui *webUI) Reload() error {
	const maxResourceFileSize = 512 * 1024
	// load favicon.ico and index.html
	res := make(map[string][]byte, 2)
	for _, name := range [...]string{
		"favicon.ico", "index.html",
	} {
		file, err := ui.hfs.Open(name)
		if err != nil {
			return errors.Errorf("failed to open %s: %s", name, err)
		}
		data, err := security.ReadAll(file, maxResourceFileSize)
		if err != nil {
			return errors.Errorf("failed to read %s: %s", name, err)
		}
		res[name] = data
	}
	ui.rwm.Lock()
	defer ui.rwm.Unlock()
	ui.favicon = res["favicon.ico"]
	ui.index = res["index.html"]
	return nil
}

func (ui *webUI) getFavicon() []byte {
	ui.rwm.RLock()
	defer ui.rwm.RUnlock()
	return ui.favicon
}

func (ui *webUI) getIndex() []byte {
	ui.rwm.RLock()
	defer ui.rwm.RUnlock()
	return ui.index
}

func (ui *webUI) handleFavicon(w http.ResponseWriter, _ *http.Request) {
	data := ui.getFavicon()
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (ui *webUI) handleIndex(w http.ResponseWriter, _ *http.Request) {
	data := ui.getIndex()
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// webAPI contain the actual handler, login and handle event.
type webAPI struct {
	ctx *Client

	adminUsername *security.Bytes
	adminPassword *security.Bytes

	maxReqBodySize      int64
	maxLargeReqBodySize int64

	users    map[string]string
	usersRWM sync.RWMutex

	encoderPool sync.Pool           // json encoder
	wsUpgrader  *websocket.Upgrader // notice event

	inShutdown int32
}

func newWebAPI(client *Client, opts *WebOptions, mux *http.ServeMux) (*webAPI, error) {
	wh := webAPI{
		ctx:                 client,
		maxReqBodySize:      opts.MaxBodySize,
		maxLargeReqBodySize: opts.MaxLargeBodySize,
	}
	// set administrator username
	adminUsername := opts.AdminUsername
	if adminUsername == "" {
		adminUsername = defaultAdminUsername
		const log = "admin username is not set, use the default username:"
		wh.log(logger.Info, log, defaultAdminUsername)
	}
	wh.adminUsername = security.NewBytes([]byte(adminUsername))
	// set administrator password
	adminPassword := opts.AdminPassword
	if adminPassword == "" {
		// generate a random password
		adminPassword = random.NewRand().String(16)
		defer security.CoverString(adminPassword)
		const log = "admin password is not set, use the random password:"
		wh.log(logger.Info, log, adminPassword)
	}
	passwordBytes := []byte(adminPassword)
	defer security.CoverBytes(passwordBytes)
	hashedPwd, err := bcrypt.GenerateFromPassword(passwordBytes, 12)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate random admin password")
	}
	wh.adminPassword = security.NewBytes(hashedPwd)
	if wh.maxReqBodySize < minRequestBodySize {
		wh.maxReqBodySize = minRequestBodySize
	}
	if wh.maxLargeReqBodySize < minRequestLargeBodySize {
		wh.maxLargeReqBodySize = minRequestLargeBodySize
	}
	if len(opts.Users) != 0 {
		wh.users = opts.Users
	} else {
		wh.users = make(map[string]string)
	}
	wh.encoderPool.New = func() interface{} {
		return json.NewEncoder(64)
	}
	wh.wsUpgrader = &websocket.Upgrader{
		HandshakeTimeout: time.Minute,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}
	// set handler

	// register router
	for path, handler := range map[string]http.HandlerFunc{
		"/login": wh.handleLogin,

		"/api/auth/logout":         wh.handleAuthenticationLogout,
		"/api/auth/token/list":     wh.handleAuthenticationTokenList,
		"/api/auth/token/generate": wh.handleAuthenticationTokenGenerate,
		"/api/auth/token/add":      wh.handleAuthenticationTokenAdd,
		"/api/auth/token/remove":   wh.handleAuthenticationTokenRemove,

		"/api/core/module/status":   wh.handleCoreModuleStatus,
		"/api/core/module/add_path": wh.handleCoreAddModulePath,
		"/api/core/module/reload":   wh.handleCoreReloadModules,
		"/api/core/thread/list":     wh.handleCoreThreadList,
		"/api/core/thread/kill":     wh.handleCoreThreadKill,
		"/api/core/global/set":      wh.handleCoreSetGlobal,
		"/api/core/global/unset":    wh.handleCoreUnsetGlobal,
		"/api/core/global/get":      wh.handleCoreGetGlobal,
		"/api/core/save":            wh.handleCoreSave,
		"/api/core/version":         wh.handleCoreVersion,

		"/api/db/status":            wh.handleDatabaseStatus,
		"/api/db/host/report":       wh.handleDatabaseReportHost,
		"/api/db/host/list":         wh.handleDatabaseHosts,
		"/api/db/host/get":          wh.handleDatabaseGetHost,
		"/api/db/host/delete":       wh.handleDatabaseDeleteHost,
		"/api/db/service/report":    wh.handleDatabaseReportService,
		"/api/db/service/list":      wh.handleDatabaseServices,
		"/api/db/service/get":       wh.handleDatabaseGetService,
		"/api/db/service/delete":    wh.handleDatabaseDeleteService,
		"/api/db/client/report":     wh.handleDatabaseReportClient,
		"/api/db/client/list":       wh.handleDatabaseClients,
		"/api/db/client/get":        wh.handleDatabaseGetClient,
		"/api/db/client/delete":     wh.handleDatabaseDeleteClient,
		"/api/db/cred/list":         wh.handleDatabaseCredentials,
		"/api/db/cred/create":       wh.handleDatabaseCreateCredential,
		"/api/db/cred/delete":       wh.handleDatabaseDeleteCredentials,
		"/api/db/loot/report":       wh.handleDatabaseReportLoot,
		"/api/db/loot/list":         wh.handleDatabaseLoots,
		"/api/db/workspace/list":    wh.handleDatabaseWorkspaces,
		"/api/db/workspace/get":     wh.handleDatabaseGetWorkspace,
		"/api/db/workspace/add":     wh.handleDatabaseAddWorkspace,
		"/api/db/workspace/delete":  wh.handleDatabaseDeleteWorkspace,
		"/api/db/workspace/set":     wh.handleDatabaseSetWorkspace,
		"/api/db/workspace/current": wh.handleDatabaseCurrentWorkspace,
		"/api/db/events":            wh.handleDatabaseEvents,
		"/api/db/import_data":       wh.handleDatabaseImportData,

		"/api/console/list":           wh.handleConsoleList,
		"/api/console/create":         wh.handleConsoleCreate,
		"/api/console/destroy":        wh.handleConsoleDestroy,
		"/api/console/read":           wh.handleConsoleRead,
		"/api/console/write":          wh.handleConsoleWrite,
		"/api/console/session_detach": wh.handleConsoleSessionDetach,
		"/api/console/session_kill":   wh.handleConsoleSessionKill,

		"/api/plugin/load":   wh.handlePluginLoad,
		"/api/plugin/unload": wh.handlePluginUnload,
		"/api/plugin/loaded": wh.handlePluginLoaded,

		"/api/module/exploits":                   wh.handleModuleExploits,
		"/api/module/auxiliary":                  wh.handleModuleAuxiliary,
		"/api/module/post":                       wh.handleModulePost,
		"/api/module/payloads":                   wh.handleModulePayloads,
		"/api/module/encoders":                   wh.handleModuleEncoders,
		"/api/module/nops":                       wh.handleModuleNops,
		"/api/module/evasion":                    wh.handleModuleEvasion,
		"/api/module/info":                       wh.handleModuleInfo,
		"/api/module/options":                    wh.handleModuleOptions,
		"/api/module/payloads/compatible":        wh.handleModuleCompatiblePayloads,
		"/api/module/payloads/target_compatible": wh.handleModuleTargetCompatiblePayloads,
		"/api/module/post/session_compatible":    wh.handleModuleCompatibleSessions,
		"/api/module/evasion/compatible":         wh.handleModuleCompatibleEvasionPayloads,
		"/api/module/evasion/target_compatible":  wh.handleModuleTargetCompatibleEvasionPayloads,
		"/api/module/formats/encode":             wh.handleModuleEncodeFormats,
		"/api/module/formats/executable":         wh.handleModuleExecutableFormats,
		"/api/module/formats/transform":          wh.handleModuleTransformFormats,
		"/api/module/formats/encryption":         wh.handleModuleEncryptionFormats,
		"/api/module/platforms":                  wh.handleModulePlatforms,
		"/api/module/architectures":              wh.handleModuleArchitectures,
		"/api/module/encode":                     wh.handleModuleEncode,
		"/api/module/generate_payload":           wh.handleModuleGeneratePayload,
		"/api/module/execute":                    wh.handleModuleExecute,
		"/api/module/check":                      wh.handleModuleCheck,
		"/api/module/running_status":             wh.handleModuleRunningStatus,

		"/api/job/list": wh.handleJobList,
		"/api/job/info": wh.handleJobInfo,
		"/api/job/stop": wh.handleJobStop,

		"/api/session/list":                       wh.handleSessionList,
		"/api/session/stop":                       wh.handleSessionStop,
		"/api/session/shell/read":                 wh.handleSessionShellRead,
		"/api/session/shell/write":                wh.handleSessionShellWrite,
		"/api/session/upgrade":                    wh.handleSessionUpgrade,
		"/api/session/meterpreter/read":           wh.handleSessionMeterpreterRead,
		"/api/session/meterpreter/write":          wh.handleSessionMeterpreterWrite,
		"/api/session/meterpreter/session_detach": wh.handleSessionMeterpreterSessionDetach,
		"/api/session/meterpreter/session_kill":   wh.handleSessionMeterpreterSessionKill,
		"/api/session/meterpreter/run_single":     wh.handleSessionMeterpreterRunSingle,
		"/api/session/compatible_modules":         wh.handleSessionCompatibleModules,
	} {

		mux.HandleFunc(path, handler)

	}

	return &wh, nil
}

func (api *webAPI) shuttingDown() bool {
	return atomic.LoadInt32(&api.inShutdown) != 0
}

func (api *webAPI) logf(lv logger.Level, format string, log ...interface{}) {
	if api.shuttingDown() {
		return
	}
	api.ctx.logger.Printf(lv, "msfrpc-web api", format, log...)
}

func (api *webAPI) log(lv logger.Level, log ...interface{}) {
	if api.shuttingDown() {
		return
	}
	api.ctx.logger.Println(lv, "msfrpc-web api", log...)
}

// callbacks is used to notice that some data is updated.
func (api *webAPI) onToken(token string, add bool) {

}

func (api *webAPI) onJob(id, name string, active bool) {

}

func (api *webAPI) onSession(id uint64, info *SessionInfo, opened bool) {

}

func (api *webAPI) onHost(workspace string, host *DBHost, add bool) {

}

func (api *webAPI) onCredential(workspace string, cred *DBCred, add bool) {

}

func (api *webAPI) onLoot(workspace string, loot *DBLoot) {

}

func (api *webAPI) onEvent(event string) {

}

type webHandler struct {
	ctx *Client

	username    string
	password    string
	maxBodySize int64
	ioInterval  time.Duration

	upgrader    *websocket.Upgrader
	encoderPool sync.Pool
}

func (api *webAPI) Close() {
	// websocket.IsWebSocketUpgrade()
	api.ctx = nil
}

func (api *webAPI) readRequest(r *http.Request, req interface{}) error {
	err := json.NewDecoder(io.LimitReader(r.Body, api.maxReqBodySize)).Decode(req)
	if err != nil {
		name := xreflect.GetStructureName(req)
		buf := httptool.PrintRequest(r)
		api.logf(logger.Error, "failed to read request about %s\n%s", name, buf)
		_, _ = io.Copy(ioutil.Discard, r.Body)
		return err
	}
	return nil
}

// readLargeRequest is used to request like upload file
func (api *webAPI) readLargeRequest(r *http.Request, req interface{}) error {
	err := json.NewDecoder(io.LimitReader(r.Body, api.maxLargeReqBodySize)).Decode(req)
	if err != nil {
		name := xreflect.GetStructureName(req)
		buf := httptool.PrintRequest(r)
		api.logf(logger.Error, "failed to read large request about %s\n%s", name, buf)
		_, _ = io.Copy(ioutil.Discard, r.Body)
		return err
	}
	return nil
}

func (api *webAPI) writeResponse(w http.ResponseWriter, resp interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	encoder := api.encoderPool.Get().(*json.Encoder)
	defer api.encoderPool.Put(encoder)
	data, err := encoder.Encode(resp)
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(data)
}

func (api *webAPI) writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	e := struct {
		Error string `json:"error"`
	}{}
	if err != nil {
		e.Error = err.Error()
	}
	encoder := api.encoderPool.Get().(*json.Encoder)
	defer api.encoderPool.Put(encoder)
	data, err := encoder.Encode(&e)
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(data)
}

func (api *webAPI) handlePanic(w http.ResponseWriter, _ *http.Request, e interface{}) {
	w.WriteHeader(http.StatusInternalServerError)

	// if is super user return the panic
	_, _ = xpanic.Print(e, "web").WriteTo(w)

	csrf.Protect(nil, nil)
	sessions.NewSession(nil, "")
}

func (api *webAPI) handleLogin(w http.ResponseWriter, r *http.Request) {
	// upgrade to websocket connection, server can push message to client
	conn, err := api.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		api.log(logger.Error, "failed to upgrade", err)
		return
	}
	_ = conn.Close()
}

func (api *webAPI) checkUser(w http.ResponseWriter, r *http.Request) {

}

// ---------------------------------------Metasploit RPC API---------------------------------------

// --------------------------------------about authentication--------------------------------------

func (api *webAPI) handleAuthenticationLogout(w http.ResponseWriter, r *http.Request) {
	req := AuthLogoutRequest{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.AuthLogout(req.Token)
	api.writeError(w, err)
}

func (api *webAPI) handleAuthenticationTokenList(w http.ResponseWriter, r *http.Request) {
	tokens, err := api.ctx.AuthTokenList(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Tokens []string `json:"tokens"`
	}{
		Tokens: tokens,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleAuthenticationTokenGenerate(w http.ResponseWriter, r *http.Request) {
	token, err := api.ctx.AuthTokenGenerate(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Token string `json:"token"`
	}{
		Token: token,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleAuthenticationTokenAdd(w http.ResponseWriter, r *http.Request) {
	req := AuthTokenAddRequest{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.AuthTokenAdd(r.Context(), req.Token)
	api.writeError(w, err)
}

func (api *webAPI) handleAuthenticationTokenRemove(w http.ResponseWriter, r *http.Request) {
	req := AuthTokenRemoveRequest{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.AuthTokenRemove(r.Context(), req.Token)
	api.writeError(w, err)
}

// -------------------------------------------about core-------------------------------------------

func (api *webAPI) handleCoreModuleStatus(w http.ResponseWriter, r *http.Request) {
	status, err := api.ctx.CoreModuleStats(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, status)
}

func (api *webAPI) handleCoreAddModulePath(w http.ResponseWriter, r *http.Request) {
	req := CoreAddModulePathRequest{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	status, err := api.ctx.CoreAddModulePath(r.Context(), req.Path)
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, status)
}

func (api *webAPI) handleCoreReloadModules(w http.ResponseWriter, r *http.Request) {
	status, err := api.ctx.CoreReloadModules(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, status)
}

func (api *webAPI) handleCoreThreadList(w http.ResponseWriter, r *http.Request) {
	list, err := api.ctx.CoreThreadList(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Threads map[uint64]*CoreThreadInfo `json:"threads"`
	}{
		Threads: list,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleCoreThreadKill(w http.ResponseWriter, r *http.Request) {
	req := CoreThreadKillRequest{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.CoreThreadKill(r.Context(), req.ID)
	api.writeError(w, err)
}

func (api *webAPI) handleCoreSetGlobal(w http.ResponseWriter, r *http.Request) {
	req := CoreSetGRequest{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.CoreSetG(r.Context(), req.Name, req.Value)
	api.writeError(w, err)
}

func (api *webAPI) handleCoreUnsetGlobal(w http.ResponseWriter, r *http.Request) {
	req := CoreUnsetGRequest{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.CoreUnsetG(r.Context(), req.Name)
	api.writeError(w, err)
}

func (api *webAPI) handleCoreGetGlobal(w http.ResponseWriter, r *http.Request) {
	req := CoreGetGRequest{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	value, err := api.ctx.CoreGetG(r.Context(), req.Name)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Value string `json:"value"`
	}{
		Value: value,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleCoreSave(w http.ResponseWriter, r *http.Request) {
	err := api.ctx.CoreSave(r.Context())
	api.writeError(w, err)
}

func (api *webAPI) handleCoreVersion(w http.ResponseWriter, r *http.Request) {
	version, err := api.ctx.CoreVersion(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, version)
}

// -----------------------------------------about database-----------------------------------------

func (api *webAPI) handleDatabaseStatus(w http.ResponseWriter, r *http.Request) {
	status, err := api.ctx.DBStatus(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, status)
}

func (api *webAPI) handleDatabaseReportHost(w http.ResponseWriter, r *http.Request) {
	req := DBReportHost{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.DBReportHost(r.Context(), &req)
	api.writeError(w, err)
}

func (api *webAPI) handleDatabaseHosts(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Workspace string `json:"workspace"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	hosts, err := api.ctx.DBHosts(r.Context(), req.Workspace)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Hosts []*DBHost `json:"hosts"`
	}{
		Hosts: hosts,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleDatabaseGetHost(w http.ResponseWriter, r *http.Request) {
	req := DBGetHostOptions{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	host, err := api.ctx.DBGetHost(r.Context(), &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, &host)
}

func (api *webAPI) handleDatabaseDeleteHost(w http.ResponseWriter, r *http.Request) {
	req := DBDelHostOptions{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	_, err = api.ctx.DBDelHost(r.Context(), &req)
	api.writeError(w, err)
}

func (api *webAPI) handleDatabaseReportService(w http.ResponseWriter, r *http.Request) {
	req := DBReportService{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.DBReportService(r.Context(), &req)
	api.writeError(w, err)
}

func (api *webAPI) handleDatabaseServices(w http.ResponseWriter, r *http.Request) {
	req := DBServicesOptions{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	services, err := api.ctx.DBServices(r.Context(), &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Services []*DBService `json:"services"`
	}{
		Services: services,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleDatabaseGetService(w http.ResponseWriter, r *http.Request) {
	req := DBGetServiceOptions{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	services, err := api.ctx.DBGetService(r.Context(), &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Services []*DBService `json:"services"`
	}{
		Services: services,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleDatabaseDeleteService(w http.ResponseWriter, r *http.Request) {
	req := DBDelServiceOptions{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	_, err = api.ctx.DBDelService(r.Context(), &req)
	api.writeError(w, err)
}

func (api *webAPI) handleDatabaseReportClient(w http.ResponseWriter, r *http.Request) {
	req := DBReportClient{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.DBReportClient(r.Context(), &req)
	api.writeError(w, err)
}

func (api *webAPI) handleDatabaseClients(w http.ResponseWriter, r *http.Request) {
	req := DBClientsOptions{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	clients, err := api.ctx.DBClients(r.Context(), &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Clients []*DBClient `json:"clients"`
	}{
		Clients: clients,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleDatabaseGetClient(w http.ResponseWriter, r *http.Request) {
	req := DBGetClientOptions{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	client, err := api.ctx.DBGetClient(r.Context(), &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, client)
}

func (api *webAPI) handleDatabaseDeleteClient(w http.ResponseWriter, r *http.Request) {
	req := DBDelClientOptions{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	_, err = api.ctx.DBDelClient(r.Context(), &req)
	api.writeError(w, err)
}

func (api *webAPI) handleDatabaseCredentials(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Workspace string `json:"workspace"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	creds, err := api.ctx.DBCreds(r.Context(), req.Workspace)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Creds []*DBCred `json:"credentials"`
	}{
		Creds: creds,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleDatabaseCreateCredential(w http.ResponseWriter, r *http.Request) {
	req := DBCreateCredentialOptions{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	_, err = api.ctx.DBCreateCredential(r.Context(), &req)
	api.writeError(w, err)
}

func (api *webAPI) handleDatabaseDeleteCredentials(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Workspace string `json:"workspace"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	_, err = api.ctx.DBDelCreds(r.Context(), req.Workspace)
	api.writeError(w, err)
}

func (api *webAPI) handleDatabaseReportLoot(w http.ResponseWriter, r *http.Request) {
	req := DBReportLoot{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.DBReportLoot(r.Context(), &req)
	api.writeError(w, err)
}

func (api *webAPI) handleDatabaseLoots(w http.ResponseWriter, r *http.Request) {
	req := DBLootsOptions{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	loots, err := api.ctx.DBLoots(r.Context(), &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, loots)
}

func (api *webAPI) handleDatabaseWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := api.ctx.DBWorkspaces(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Workspaces []*DBWorkspace `json:"workspaces"`
	}{
		Workspaces: workspaces,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleDatabaseGetWorkspace(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	workspace, err := api.ctx.DBGetWorkspace(r.Context(), req.Name)
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, workspace)
}

func (api *webAPI) handleDatabaseAddWorkspace(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.DBAddWorkspace(r.Context(), req.Name)
	api.writeError(w, err)
}

func (api *webAPI) handleDatabaseDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.DBDelWorkspace(r.Context(), req.Name)
	api.writeError(w, err)
}

func (api *webAPI) handleDatabaseSetWorkspace(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.DBSetWorkspace(r.Context(), req.Name)
	api.writeError(w, err)
}

func (api *webAPI) handleDatabaseCurrentWorkspace(w http.ResponseWriter, r *http.Request) {
	result, err := api.ctx.DBCurrentWorkspace(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, result)
}

func (api *webAPI) handleDatabaseEvents(w http.ResponseWriter, r *http.Request) {
	req := DBEventOptions{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	events, err := api.ctx.DBEvent(r.Context(), &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Events []*DBEvent `json:"events"`
	}{
		Events: events,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleDatabaseImportData(w http.ResponseWriter, r *http.Request) {
	req := DBImportDataOptions{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.DBImportData(r.Context(), &req)
	api.writeError(w, err)
}

// ------------------------------------------about console-----------------------------------------

func (api *webAPI) handleConsoleList(w http.ResponseWriter, r *http.Request) {
	consoles, err := api.ctx.ConsoleList(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Console []*ConsoleInfo `json:"consoles"`
	}{
		Console: consoles,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleConsoleCreate(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Workspace  string        `json:"workspace"`
		IOInterval time.Duration `json:"io_interval"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	if req.IOInterval < 1 {
		req.IOInterval = minReadInterval
	}

	console, err := api.ctx.NewConsole(r.Context(), req.Workspace, req.IOInterval)
	if err != nil {
		api.writeError(w, err)
		return
	}
	_ = console.Close()
}

func (api *webAPI) handleConsoleDestroy(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	// first check is in web handler
	err = api.ctx.ConsoleDestroy(r.Context(), req.ID)
	api.writeError(w, err)
}

func (api *webAPI) handleConsoleRead(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	// first check is in web handler
	err = api.ctx.ConsoleSessionDetach(r.Context(), req.ID)
	api.writeError(w, err)
}

func (api *webAPI) handleConsoleWrite(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	// first check is in web handler
	err = api.ctx.ConsoleSessionKill(r.Context(), req.ID)
	api.writeError(w, err)
}

func (api *webAPI) handleConsoleSessionDetach(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	// first check is in web handler
	err = api.ctx.ConsoleSessionDetach(r.Context(), req.ID)
	api.writeError(w, err)
}

func (api *webAPI) handleConsoleSessionKill(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	// first check is in web handler
	err = api.ctx.ConsoleSessionKill(r.Context(), req.ID)
	api.writeError(w, err)
}

// ------------------------------------------about plugin------------------------------------------

func (api *webAPI) handlePluginLoad(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Name    string            `json:"name"`
		Options map[string]string `json:"options"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.PluginLoad(r.Context(), req.Name, req.Options)
	api.writeError(w, err)
}

func (api *webAPI) handlePluginUnload(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.PluginUnload(r.Context(), req.Name)
	api.writeError(w, err)
}

func (api *webAPI) handlePluginLoaded(w http.ResponseWriter, r *http.Request) {
	plugins, err := api.ctx.PluginLoaded(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Plugins []string `json:"plugins"`
	}{
		Plugins: plugins,
	}
	api.writeResponse(w, &resp)
}

// ------------------------------------------about module------------------------------------------

func (api *webAPI) handleModuleExploits(w http.ResponseWriter, r *http.Request) {
	modules, err := api.ctx.ModuleExploits(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleAuxiliary(w http.ResponseWriter, r *http.Request) {
	modules, err := api.ctx.ModuleAuxiliary(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModulePost(w http.ResponseWriter, r *http.Request) {
	modules, err := api.ctx.ModulePost(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModulePayloads(w http.ResponseWriter, r *http.Request) {
	modules, err := api.ctx.ModulePayloads(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleEncoders(w http.ResponseWriter, r *http.Request) {
	modules, err := api.ctx.ModuleEncoders(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleNops(w http.ResponseWriter, r *http.Request) {
	modules, err := api.ctx.ModuleNops(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleEvasion(w http.ResponseWriter, r *http.Request) {
	modules, err := api.ctx.ModuleEvasion(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleInfo(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	info, err := api.ctx.ModuleInfo(r.Context(), req.Type, req.Name)
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, info)
}

func (api *webAPI) handleModuleOptions(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	opts, err := api.ctx.ModuleOptions(r.Context(), req.Type, req.Name)
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, opts)
}

func (api *webAPI) handleModuleCompatiblePayloads(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	payloads, err := api.ctx.ModuleCompatiblePayloads(r.Context(), req.Name)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Payloads []string `json:"payloads"`
	}{
		Payloads: payloads,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleTargetCompatiblePayloads(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Name   string `json:"name"`
		Target uint64 `json:"target"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	payloads, err := api.ctx.ModuleTargetCompatiblePayloads(r.Context(), req.Name, req.Target)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Payloads []string `json:"payloads"`
	}{
		Payloads: payloads,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleCompatibleSessions(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	cSessions, err := api.ctx.ModuleCompatibleSessions(r.Context(), req.Name)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Sessions []string `json:"sessions"`
	}{
		Sessions: cSessions,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleCompatibleEvasionPayloads(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	payloads, err := api.ctx.ModuleCompatibleEvasionPayloads(r.Context(), req.Name)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Payloads []string `json:"payloads"`
	}{
		Payloads: payloads,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleTargetCompatibleEvasionPayloads(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Name   string `json:"name"`
		Target uint64 `json:"target"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	payloads, err := api.ctx.ModuleTargetCompatibleEvasionPayloads(r.Context(), req.Name, req.Target)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Payloads []string `json:"payloads"`
	}{
		Payloads: payloads,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleEncodeFormats(w http.ResponseWriter, r *http.Request) {
	formats, err := api.ctx.ModuleEncodeFormats(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Formats []string `json:"formats"`
	}{
		Formats: formats,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleExecutableFormats(w http.ResponseWriter, r *http.Request) {
	formats, err := api.ctx.ModuleExecutableFormats(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Formats []string `json:"formats"`
	}{
		Formats: formats,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleTransformFormats(w http.ResponseWriter, r *http.Request) {
	formats, err := api.ctx.ModuleTransformFormats(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Formats []string `json:"formats"`
	}{
		Formats: formats,
	}
	api.writeResponse(w, &resp)
}
func (api *webAPI) handleModuleEncryptionFormats(w http.ResponseWriter, r *http.Request) {
	formats, err := api.ctx.ModuleEncryptionFormats(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Formats []string `json:"formats"`
	}{
		Formats: formats,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModulePlatforms(w http.ResponseWriter, r *http.Request) {
	platforms, err := api.ctx.ModulePlatforms(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Platforms []string `json:"platforms"`
	}{
		Platforms: platforms,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleArchitectures(w http.ResponseWriter, r *http.Request) {
	architectures, err := api.ctx.ModuleArchitectures(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Architectures []string `json:"architectures"`
	}{
		Architectures: architectures,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleEncode(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Data    string               `json:"data"`
		Encoder string               `json:"encoder"`
		Options *ModuleEncodeOptions `json:"options"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	data, err := api.ctx.ModuleEncode(r.Context(), req.Data, req.Encoder, req.Options)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Data string `json:"data"`
	}{
		Data: data,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleGeneratePayload(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Name    string                `json:"name"`
		Options *ModuleExecuteOptions `json:"options"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	result, err := api.ctx.ModuleExecute(r.Context(), "payload", req.Name, req.Options)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Payload string `json:"payload"`
	}{
		Payload: result.Payload,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleExecute(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Type    string                 `json:"type"`
		Name    string                 `json:"name"`
		Options map[string]interface{} `json:"options"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	result, err := api.ctx.ModuleExecute(r.Context(), req.Type, req.Name, req.Options)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		JobID uint64 `json:"job_id"`
		UUID  string `json:"uuid"`
	}{
		JobID: result.JobID,
		UUID:  result.UUID,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleModuleCheck(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Type    string                 `json:"type"`
		Name    string                 `json:"name"`
		Options map[string]interface{} `json:"options"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	result, err := api.ctx.ModuleCheck(r.Context(), req.Type, req.Name, req.Options)
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, result)
}

func (api *webAPI) handleModuleRunningStatus(w http.ResponseWriter, r *http.Request) {
	status, err := api.ctx.ModuleRunningStats(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, status)
}

// -------------------------------------------about job--------------------------------------------

func (api *webAPI) handleJobList(w http.ResponseWriter, r *http.Request) {
	jobs, err := api.ctx.JobList(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Jobs map[string]string `json:"jobs"`
	}{
		Jobs: jobs,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleJobInfo(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	info, err := api.ctx.JobInfo(r.Context(), req.ID)
	if err != nil {
		api.writeError(w, err)
		return
	}
	api.writeResponse(w, info)
}

func (api *webAPI) handleJobStop(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.JobStop(r.Context(), req.ID)
	api.writeError(w, err)
}

// -----------------------------------------about session------------------------------------------

func (api *webAPI) handleSessionList(w http.ResponseWriter, r *http.Request) {
	list, err := api.ctx.SessionList(r.Context())
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Sessions map[uint64]*SessionInfo `json:"sessions"`
	}{
		Sessions: list,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleSessionStop(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID uint64 `json:"id"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	// first check is in web handler
	err = api.ctx.SessionStop(r.Context(), req.ID)
	api.writeError(w, err)
}

func (api *webAPI) handleSessionShellRead(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID uint64 `json:"id"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	result, err := api.ctx.SessionShellRead(r.Context(), req.ID)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Data string `json:"data"`
	}{
		Data: result.Data,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleSessionShellWrite(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID   uint64 `json:"id"`
		Data string `json:"data"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	// check
	_, err = api.ctx.SessionShellWrite(r.Context(), req.ID, req.Data)
	api.writeError(w, err)
}

func (api *webAPI) handleSessionUpgrade(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID      uint64                 `json:"id"`
		Host    string                 `json:"host"`
		Port    uint64                 `json:"port"`
		Options map[string]interface{} `json:"options"`
		Wait    time.Duration          `json:"wait"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	result, err := api.ctx.SessionUpgrade(r.Context(),
		req.ID, req.Host, req.Port, req.Options, req.Wait)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		JobID uint64 `json:"job_id"`
	}{
		JobID: result.JobID,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleSessionMeterpreterRead(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID uint64 `json:"id"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	data, err := api.ctx.SessionMeterpreterRead(r.Context(), req.ID)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Data string `json:"data"`
	}{
		Data: data,
	}
	api.writeResponse(w, &resp)
}

func (api *webAPI) handleSessionMeterpreterWrite(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID   uint64 `json:"id"`
		Data string `json:"data"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.SessionMeterpreterWrite(r.Context(), req.ID, req.Data)
	api.writeError(w, err)
}

func (api *webAPI) handleSessionMeterpreterSessionDetach(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID uint64 `json:"id"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	// check exist
	err = api.ctx.SessionMeterpreterSessionDetach(r.Context(), req.ID)
	api.writeError(w, err)
}

func (api *webAPI) handleSessionMeterpreterSessionKill(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID uint64 `json:"id"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	// check exist
	err = api.ctx.SessionMeterpreterSessionKill(r.Context(), req.ID)
	api.writeError(w, err)
}

func (api *webAPI) handleSessionMeterpreterRunSingle(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID      uint64 `json:"id"`
		Command string `json:"command"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	err = api.ctx.SessionMeterpreterRunSingle(r.Context(), req.ID, req.Command)
	api.writeError(w, err)
}

func (api *webAPI) handleSessionCompatibleModules(w http.ResponseWriter, r *http.Request) {
	req := struct {
		ID uint64 `json:"id"`
	}{}
	err := api.readRequest(r, &req)
	if err != nil {
		api.writeError(w, err)
		return
	}
	modules, err := api.ctx.SessionCompatibleModules(r.Context(), req.ID)
	if err != nil {
		api.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	api.writeResponse(w, &resp)
}
