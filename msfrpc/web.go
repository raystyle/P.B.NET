package msfrpc

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/netutil"

	"project/internal/logger"
	"project/internal/option"
	"project/internal/patch/json"
	"project/internal/virtualconn"
	"project/internal/xpanic"
)

// subHTTPFileSystem is used to open sub directory for http file server.
type subHTTPFileSystem struct {
	hfs  http.FileSystem
	path string
}

func newSubHTTPFileSystem(hfs http.FileSystem, path string) *subHTTPFileSystem {
	return &subHTTPFileSystem{hfs: hfs, path: path + "/"}
}

func (s *subHTTPFileSystem) Open(name string) (http.File, error) {
	return s.hfs.Open(s.path + name)
}

// parallelReader is used to wrap Console, Shell and Meterpreter.
// different reader can get the same data.
type parallelReader struct {
	rc     io.ReadCloser
	logger logger.Logger
	onRead func()

	// store history output
	buf bytes.Buffer
	rwm sync.RWMutex
}

func newParallelReader(rc io.ReadCloser, logger logger.Logger, onRead func()) *parallelReader {
	reader := parallelReader{
		rc:     rc,
		logger: logger,
		onRead: onRead,
	}
	go reader.readLoop()
	return &reader
}

func (pr *parallelReader) readLoop() {
	defer func() {
		if r := recover(); r != nil {
			b := xpanic.Print(r, "parallelReader.readLoop")
			pr.logger.Println(logger.Fatal, "parallelReader", b)
			// restart readLoop
			time.Sleep(time.Second)
			go pr.readLoop()
		}
	}()
	var (
		n   int
		err error
	)
	buf := make([]byte, 4096)
	for {
		n, err = pr.rc.Read(buf)
		if err != nil {
			return
		}
		pr.writeToBuffer(buf[:n])
		pr.onRead()
	}
}

func (pr *parallelReader) writeToBuffer(b []byte) {
	pr.rwm.Lock()
	defer pr.rwm.Unlock()
	pr.buf.Write(b)
}

// Bytes is used to get buffer data.
func (pr *parallelReader) Bytes(start int) []byte {
	if start < 0 {
		start = 0
	}
	pr.rwm.RLock()
	defer pr.rwm.RUnlock()
	l := pr.buf.Len()
	if start > l {
		return nil
	}
	b := make([]byte, l-start)
	copy(b, pr.buf.Bytes()[start:])
	return b
}

// Close is used to close parallel reader.
func (pr *parallelReader) Close() error {
	return pr.rc.Close()
}

const minRequestBodySize = 1 << 20 // 1MB

// WebServerOptions contains options about web server.
type WebServerOptions struct {
	option.HTTPServer
	MaxConns int
	// incoming request body size
	MaxBodySize int64
	// about Console, Shell and Meterpreter IO interval
	IOInterval time.Duration
}

// WebServer is provide a web UI.
type WebServer struct {
	maxConns int

	server *http.Server

	handler *webHandler
}

// NewWebServer is used to create a web server.
func (msf *MSFRPC) NewWebServer(
	username string,
	password string,
	hfs http.FileSystem,
	opts *WebServerOptions,
) (*WebServer, error) {
	httpServer, err := opts.HTTPServer.Apply()
	if err != nil {
		return nil, err
	}
	// configure web handler.
	wh := webHandler{
		ctx:         msf,
		maxBodySize: opts.MaxBodySize,
	}
	if wh.maxBodySize < minRequestBodySize { // 1 MB
		wh.maxBodySize = minRequestBodySize
	}
	wh.upgrader = &websocket.Upgrader{
		HandshakeTimeout: time.Minute,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}
	wh.encoderPool.New = func() interface{} {
		return json.NewEncoder(64)
	}
	// configure router
	router := &httprouter.Router{
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		PanicHandler:           wh.handlePanic,
	}
	// resource
	router.ServeFiles("/css/*filepath", newSubHTTPFileSystem(hfs, "css"))
	router.ServeFiles("/js/*filepath", newSubHTTPFileSystem(hfs, "js"))
	router.ServeFiles("/img/*filepath", newSubHTTPFileSystem(hfs, "img"))
	// favicon.ico
	router.GET("/favicon.ico", func(w hRW, _ *hR, _ hP) {
		file, err := hfs.Open("favicon.ico")
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("404 not found"))
			return
		}
		_, _ = io.Copy(w, file)
	})
	// index.html
	router.GET("/", func(w hRW, _ *hR, _ hP) {
		file, err := hfs.Open("index.html")
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("404 not found"))
			return
		}
		_, _ = io.Copy(w, file)
	})
	// register router
	for path, handler := range map[string]httprouter.Handle{
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
		router.GET(path, handler)
		router.POST(path, handler)
	}
	// set web server
	httpServer.Handler = router
	httpServer.ErrorLog = logger.Wrap(logger.Warning, "web", msf.logger)
	web := WebServer{
		server:  httpServer,
		handler: &wh,
	}
	if web.maxConns < 32 {
		web.maxConns = 1000
	}
	return &web, nil
}

// Callbacks is used to return callbacks for monitor.
func (web *WebServer) Callbacks() *Callbacks {
	return &Callbacks{
		OnToken:      web.handler.onToken,
		OnJob:        web.handler.onJob,
		OnSession:    web.handler.onSession,
		OnHost:       web.handler.onHost,
		OnCredential: web.handler.onCredential,
		OnLoot:       web.handler.onLoot,
		OnEvent:      web.handler.onEvent,
	}
}

// Serve is used to start web server.
func (web *WebServer) Serve(listener net.Listener) error {
	l := netutil.LimitListener(listener, web.maxConns)
	switch listener.(type) {
	case *virtualconn.Listener:
		return web.server.Serve(l)
	default:
		return web.server.ServeTLS(l, "", "")
	}
}

// Close is used to close web server.
func (web *WebServer) Close() error {
	return web.server.Close()
}

// shortcut about interface and structure.
type hRW = http.ResponseWriter
type hR = http.Request
type hP = httprouter.Params

type webHandler struct {
	ctx *MSFRPC

	username    string
	password    string
	maxBodySize int64
	ioInterval  time.Duration

	upgrader    *websocket.Upgrader
	encoderPool sync.Pool
}

func (wh *webHandler) Close() {
	wh.ctx = nil
}

func (wh *webHandler) logf(lv logger.Level, format string, log ...interface{}) {
	wh.ctx.logger.Printf(lv, "web", format, log...)
}

func (wh *webHandler) log(lv logger.Level, log ...interface{}) {
	wh.ctx.logger.Println(lv, "web", log...)
}

func (wh *webHandler) readRequest(r *hR, req interface{}) error {
	return json.NewDecoder(io.LimitReader(r.Body, wh.maxBodySize)).Decode(req)
}

func (wh *webHandler) writeResponse(w hRW, resp interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	encoder := wh.encoderPool.Get().(*json.Encoder)
	defer wh.encoderPool.Put(encoder)
	data, err := encoder.Encode(resp)
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(data)
}

func (wh *webHandler) writeError(w hRW, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	e := struct {
		Error string `json:"error"`
	}{}
	if err != nil {
		e.Error = err.Error()
	}
	encoder := wh.encoderPool.Get().(*json.Encoder)
	defer wh.encoderPool.Put(encoder)
	data, err := encoder.Encode(&e)
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(data)
}

func (wh *webHandler) handlePanic(w hRW, _ *hR, e interface{}) {
	w.WriteHeader(http.StatusInternalServerError)

	// if is super user return the panic
	_, _ = xpanic.Print(e, "web").WriteTo(w)

	csrf.Protect(nil, nil)
	sessions.NewSession(nil, "")
}

func (wh *webHandler) handleLogin(w hRW, r *hR, _ hP) {
	// upgrade to websocket connection, server can push message to client
	conn, err := wh.upgrader.Upgrade(w, r, nil)
	if err != nil {
		wh.log(logger.Error, "failed to upgrade", err)
		return
	}
	_ = conn.Close()
}

func (wh *webHandler) checkUser(w hRW, r *hR, _ hP) {

}

// callbacks is used to notice web UI that some data is updated.

func (wh *webHandler) onToken(token string, add bool) {

}

func (wh *webHandler) onJob(id, name string, active bool) {

}

func (wh *webHandler) onSession(id uint64, info *SessionInfo, opened bool) {

}

func (wh *webHandler) onHost(workspace string, host *DBHost, add bool) {

}

func (wh *webHandler) onCredential(workspace string, cred *DBCred, add bool) {

}

func (wh *webHandler) onLoot(workspace string, loot *DBLoot) {

}

func (wh *webHandler) onEvent(event string) {

}

// ---------------------------------------Metasploit RPC API---------------------------------------

// --------------------------------------about authentication--------------------------------------

func (wh *webHandler) handleAuthenticationLogout(w hRW, r *hR, _ hP) {
	req := AuthLogoutRequest{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.AuthLogout(req.Token)
	wh.writeError(w, err)
}

func (wh *webHandler) handleAuthenticationTokenList(w hRW, r *hR, _ hP) {
	tokens, err := wh.ctx.AuthTokenList(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Tokens []string `json:"tokens"`
	}{
		Tokens: tokens,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleAuthenticationTokenGenerate(w hRW, r *hR, _ hP) {
	token, err := wh.ctx.AuthTokenGenerate(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Token string `json:"token"`
	}{
		Token: token,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleAuthenticationTokenAdd(w hRW, r *hR, _ hP) {
	req := AuthTokenAddRequest{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.AuthTokenAdd(r.Context(), req.Token)
	wh.writeError(w, err)
}

func (wh *webHandler) handleAuthenticationTokenRemove(w hRW, r *hR, _ hP) {
	req := AuthTokenRemoveRequest{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.AuthTokenRemove(r.Context(), req.Token)
	wh.writeError(w, err)
}

// -------------------------------------------about core-------------------------------------------

func (wh *webHandler) handleCoreModuleStatus(w hRW, r *hR, _ hP) {
	status, err := wh.ctx.CoreModuleStats(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, status)
}

func (wh *webHandler) handleCoreAddModulePath(w hRW, r *hR, _ hP) {
	req := CoreAddModulePathRequest{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	status, err := wh.ctx.CoreAddModulePath(r.Context(), req.Path)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, status)
}

func (wh *webHandler) handleCoreReloadModules(w hRW, r *hR, _ hP) {
	status, err := wh.ctx.CoreReloadModules(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, status)
}

func (wh *webHandler) handleCoreThreadList(w hRW, r *hR, _ hP) {
	list, err := wh.ctx.CoreThreadList(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Threads map[uint64]*CoreThreadInfo `json:"threads"`
	}{
		Threads: list,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleCoreThreadKill(w hRW, r *hR, _ hP) {
	req := CoreThreadKillRequest{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.CoreThreadKill(r.Context(), req.ID)
	wh.writeError(w, err)
}

func (wh *webHandler) handleCoreSetGlobal(w hRW, r *hR, _ hP) {
	req := CoreSetGRequest{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.CoreSetG(r.Context(), req.Name, req.Value)
	wh.writeError(w, err)
}

func (wh *webHandler) handleCoreUnsetGlobal(w hRW, r *hR, _ hP) {
	req := CoreUnsetGRequest{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.CoreUnsetG(r.Context(), req.Name)
	wh.writeError(w, err)
}

func (wh *webHandler) handleCoreGetGlobal(w hRW, r *hR, _ hP) {
	req := CoreGetGRequest{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	value, err := wh.ctx.CoreGetG(r.Context(), req.Name)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Value string `json:"value"`
	}{
		Value: value,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleCoreSave(w hRW, r *hR, _ hP) {
	err := wh.ctx.CoreSave(r.Context())
	wh.writeError(w, err)
}

func (wh *webHandler) handleCoreVersion(w hRW, r *hR, _ hP) {
	version, err := wh.ctx.CoreVersion(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, version)
}

// -----------------------------------------about database-----------------------------------------

func (wh *webHandler) handleDatabaseStatus(w hRW, r *hR, _ hP) {
	status, err := wh.ctx.DBStatus(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, status)
}

func (wh *webHandler) handleDatabaseReportHost(w hRW, r *hR, _ hP) {
	req := DBReportHost{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.DBReportHost(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDatabaseHosts(w hRW, r *hR, _ hP) {
	req := struct {
		Workspace string `json:"workspace"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	hosts, err := wh.ctx.DBHosts(r.Context(), req.Workspace)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Hosts []*DBHost `json:"hosts"`
	}{
		Hosts: hosts,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleDatabaseGetHost(w hRW, r *hR, _ hP) {
	req := DBGetHostOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	host, err := wh.ctx.DBGetHost(r.Context(), &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, &host)
}

func (wh *webHandler) handleDatabaseDeleteHost(w hRW, r *hR, _ hP) {
	req := DBDelHostOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	_, err = wh.ctx.DBDelHost(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDatabaseReportService(w hRW, r *hR, _ hP) {
	req := DBReportService{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.DBReportService(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDatabaseServices(w hRW, r *hR, _ hP) {
	req := DBServicesOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	services, err := wh.ctx.DBServices(r.Context(), &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Services []*DBService `json:"services"`
	}{
		Services: services,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleDatabaseGetService(w hRW, r *hR, _ hP) {
	req := DBGetServiceOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	services, err := wh.ctx.DBGetService(r.Context(), &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Services []*DBService `json:"services"`
	}{
		Services: services,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleDatabaseDeleteService(w hRW, r *hR, _ hP) {
	req := DBDelServiceOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	_, err = wh.ctx.DBDelService(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDatabaseReportClient(w hRW, r *hR, _ hP) {
	req := DBReportClient{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.DBReportClient(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDatabaseClients(w hRW, r *hR, _ hP) {
	req := DBClientsOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	clients, err := wh.ctx.DBClients(r.Context(), &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Clients []*DBClient `json:"clients"`
	}{
		Clients: clients,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleDatabaseGetClient(w hRW, r *hR, _ hP) {
	req := DBGetClientOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	client, err := wh.ctx.DBGetClient(r.Context(), &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, client)
}

func (wh *webHandler) handleDatabaseDeleteClient(w hRW, r *hR, _ hP) {
	req := DBDelClientOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	_, err = wh.ctx.DBDelClient(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDatabaseCredentials(w hRW, r *hR, _ hP) {
	req := struct {
		Workspace string `json:"workspace"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	creds, err := wh.ctx.DBCreds(r.Context(), req.Workspace)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Creds []*DBCred `json:"credentials"`
	}{
		Creds: creds,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleDatabaseCreateCredential(w hRW, r *hR, _ hP) {
	req := DBCreateCredentialOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	_, err = wh.ctx.DBCreateCredential(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDatabaseDeleteCredentials(w hRW, r *hR, _ hP) {
	req := struct {
		Workspace string `json:"workspace"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	_, err = wh.ctx.DBDelCreds(r.Context(), req.Workspace)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDatabaseReportLoot(w hRW, r *hR, _ hP) {
	req := DBReportLoot{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.DBReportLoot(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDatabaseLoots(w hRW, r *hR, _ hP) {
	req := DBLootsOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	loots, err := wh.ctx.DBLoots(r.Context(), &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, loots)
}

func (wh *webHandler) handleDatabaseWorkspaces(w hRW, r *hR, _ hP) {
	workspaces, err := wh.ctx.DBWorkspaces(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Workspaces []*DBWorkspace `json:"workspaces"`
	}{
		Workspaces: workspaces,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleDatabaseGetWorkspace(w hRW, r *hR, _ hP) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	workspace, err := wh.ctx.DBGetWorkspace(r.Context(), req.Name)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, workspace)
}

func (wh *webHandler) handleDatabaseAddWorkspace(w hRW, r *hR, _ hP) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.DBAddWorkspace(r.Context(), req.Name)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDatabaseDeleteWorkspace(w hRW, r *hR, _ hP) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.DBDelWorkspace(r.Context(), req.Name)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDatabaseSetWorkspace(w hRW, r *hR, _ hP) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.DBSetWorkspace(r.Context(), req.Name)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDatabaseCurrentWorkspace(w hRW, r *hR, _ hP) {
	result, err := wh.ctx.DBCurrentWorkspace(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, result)
}

func (wh *webHandler) handleDatabaseEvents(w hRW, r *hR, _ hP) {
	req := DBEventOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	events, err := wh.ctx.DBEvent(r.Context(), &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Events []*DBEvent `json:"events"`
	}{
		Events: events,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleDatabaseImportData(w hRW, r *hR, _ hP) {
	req := DBImportDataOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.DBImportData(r.Context(), &req)
	wh.writeError(w, err)
}

// ------------------------------------------about console-----------------------------------------

func (wh *webHandler) handleConsoleList(w hRW, r *hR, _ hP) {
	consoles, err := wh.ctx.ConsoleList(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Console []*ConsoleInfo `json:"consoles"`
	}{
		Console: consoles,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleConsoleCreate(w hRW, r *hR, _ hP) {
	req := struct {
		Workspace  string        `json:"workspace"`
		IOInterval time.Duration `json:"io_interval"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	if req.IOInterval < 1 {
		req.IOInterval = wh.ioInterval
	}
	console, err := wh.ctx.NewConsole(r.Context(), req.Workspace, req.IOInterval)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	_ = console.Close()
}

func (wh *webHandler) handleConsoleDestroy(w hRW, r *hR, _ hP) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	// first check is in web handler
	err = wh.ctx.ConsoleDestroy(r.Context(), req.ID)
	wh.writeError(w, err)
}

func (wh *webHandler) handleConsoleRead(w hRW, r *hR, _ hP) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	// first check is in web handler
	err = wh.ctx.ConsoleSessionDetach(r.Context(), req.ID)
	wh.writeError(w, err)
}

func (wh *webHandler) handleConsoleWrite(w hRW, r *hR, _ hP) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	// first check is in web handler
	err = wh.ctx.ConsoleSessionKill(r.Context(), req.ID)
	wh.writeError(w, err)
}

func (wh *webHandler) handleConsoleSessionDetach(w hRW, r *hR, _ hP) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	// first check is in web handler
	err = wh.ctx.ConsoleSessionDetach(r.Context(), req.ID)
	wh.writeError(w, err)
}

func (wh *webHandler) handleConsoleSessionKill(w hRW, r *hR, _ hP) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	// first check is in web handler
	err = wh.ctx.ConsoleSessionKill(r.Context(), req.ID)
	wh.writeError(w, err)
}

// ------------------------------------------about plugin------------------------------------------

func (wh *webHandler) handlePluginLoad(w hRW, r *hR, _ hP) {
	req := struct {
		Name    string            `json:"name"`
		Options map[string]string `json:"options"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.PluginLoad(r.Context(), req.Name, req.Options)
	wh.writeError(w, err)
}

func (wh *webHandler) handlePluginUnload(w hRW, r *hR, _ hP) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.PluginUnload(r.Context(), req.Name)
	wh.writeError(w, err)
}

func (wh *webHandler) handlePluginLoaded(w hRW, r *hR, _ hP) {
	plugins, err := wh.ctx.PluginLoaded(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Plugins []string `json:"plugins"`
	}{
		Plugins: plugins,
	}
	wh.writeResponse(w, &resp)
}

// ------------------------------------------about module------------------------------------------

func (wh *webHandler) handleModuleExploits(w hRW, r *hR, _ hP) {
	modules, err := wh.ctx.ModuleExploits(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleAuxiliary(w hRW, r *hR, _ hP) {
	modules, err := wh.ctx.ModuleAuxiliary(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModulePost(w hRW, r *hR, _ hP) {
	modules, err := wh.ctx.ModulePost(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModulePayloads(w hRW, r *hR, _ hP) {
	modules, err := wh.ctx.ModulePayloads(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleEncoders(w hRW, r *hR, _ hP) {
	modules, err := wh.ctx.ModuleEncoders(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleNops(w hRW, r *hR, _ hP) {
	modules, err := wh.ctx.ModuleNops(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleEvasion(w hRW, r *hR, _ hP) {
	modules, err := wh.ctx.ModuleEvasion(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleInfo(w hRW, r *hR, _ hP) {
	req := struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	info, err := wh.ctx.ModuleInfo(r.Context(), req.Type, req.Name)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, info)
}

func (wh *webHandler) handleModuleOptions(w hRW, r *hR, _ hP) {
	req := struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	opts, err := wh.ctx.ModuleOptions(r.Context(), req.Type, req.Name)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, opts)
}

func (wh *webHandler) handleModuleCompatiblePayloads(w hRW, r *hR, _ hP) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	payloads, err := wh.ctx.ModuleCompatiblePayloads(r.Context(), req.Name)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Payloads []string `json:"payloads"`
	}{
		Payloads: payloads,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleTargetCompatiblePayloads(w hRW, r *hR, _ hP) {
	req := struct {
		Name   string `json:"name"`
		Target uint64 `json:"target"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	payloads, err := wh.ctx.ModuleTargetCompatiblePayloads(r.Context(), req.Name, req.Target)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Payloads []string `json:"payloads"`
	}{
		Payloads: payloads,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleCompatibleSessions(w hRW, r *hR, _ hP) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	cSessions, err := wh.ctx.ModuleCompatibleSessions(r.Context(), req.Name)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Sessions []string `json:"sessions"`
	}{
		Sessions: cSessions,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleCompatibleEvasionPayloads(w hRW, r *hR, _ hP) {
	req := struct {
		Name string `json:"name"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	payloads, err := wh.ctx.ModuleCompatibleEvasionPayloads(r.Context(), req.Name)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Payloads []string `json:"payloads"`
	}{
		Payloads: payloads,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleTargetCompatibleEvasionPayloads(w hRW, r *hR, _ hP) {
	req := struct {
		Name   string `json:"name"`
		Target uint64 `json:"target"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	payloads, err := wh.ctx.ModuleTargetCompatibleEvasionPayloads(r.Context(), req.Name, req.Target)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Payloads []string `json:"payloads"`
	}{
		Payloads: payloads,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleEncodeFormats(w hRW, r *hR, _ hP) {
	formats, err := wh.ctx.ModuleEncodeFormats(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Formats []string `json:"formats"`
	}{
		Formats: formats,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleExecutableFormats(w hRW, r *hR, _ hP) {
	formats, err := wh.ctx.ModuleExecutableFormats(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Formats []string `json:"formats"`
	}{
		Formats: formats,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleTransformFormats(w hRW, r *hR, _ hP) {
	formats, err := wh.ctx.ModuleTransformFormats(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Formats []string `json:"formats"`
	}{
		Formats: formats,
	}
	wh.writeResponse(w, &resp)
}
func (wh *webHandler) handleModuleEncryptionFormats(w hRW, r *hR, _ hP) {
	formats, err := wh.ctx.ModuleEncryptionFormats(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Formats []string `json:"formats"`
	}{
		Formats: formats,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModulePlatforms(w hRW, r *hR, _ hP) {
	platforms, err := wh.ctx.ModulePlatforms(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Platforms []string `json:"platforms"`
	}{
		Platforms: platforms,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleArchitectures(w hRW, r *hR, _ hP) {
	architectures, err := wh.ctx.ModuleArchitectures(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Architectures []string `json:"architectures"`
	}{
		Architectures: architectures,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleEncode(w hRW, r *hR, _ hP) {
	req := struct {
		Data    string               `json:"data"`
		Encoder string               `json:"encoder"`
		Options *ModuleEncodeOptions `json:"options"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	data, err := wh.ctx.ModuleEncode(r.Context(), req.Data, req.Encoder, req.Options)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Data string `json:"data"`
	}{
		Data: data,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleGeneratePayload(w hRW, r *hR, _ hP) {
	req := struct {
		Type    string                `json:"type"`
		Name    string                `json:"name"`
		Options *ModuleExecuteOptions `json:"options"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	result, err := wh.ctx.ModuleExecute(r.Context(), req.Type, req.Name, req.Options)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Payload string `json:"payload"`
	}{
		Payload: result.Payload,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleExecute(w hRW, r *hR, _ hP) {
	req := struct {
		Type    string                 `json:"type"`
		Name    string                 `json:"name"`
		Options map[string]interface{} `json:"options"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	result, err := wh.ctx.ModuleExecute(r.Context(), req.Type, req.Name, req.Options)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		JobID uint64 `json:"job_id"`
		UUID  string `json:"uuid"`
	}{
		JobID: result.JobID,
		UUID:  result.UUID,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleModuleCheck(w hRW, r *hR, _ hP) {
	req := struct {
		Type    string                 `json:"type"`
		Name    string                 `json:"name"`
		Options map[string]interface{} `json:"options"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	result, err := wh.ctx.ModuleCheck(r.Context(), req.Type, req.Name, req.Options)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, result)
}

func (wh *webHandler) handleModuleRunningStatus(w hRW, r *hR, _ hP) {
	status, err := wh.ctx.ModuleRunningStats(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, status)
}

// -------------------------------------------about job--------------------------------------------

func (wh *webHandler) handleJobList(w hRW, r *hR, _ hP) {
	jobs, err := wh.ctx.JobList(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Jobs map[string]string `json:"jobs"`
	}{
		Jobs: jobs,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleJobInfo(w hRW, r *hR, _ hP) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	info, err := wh.ctx.JobInfo(r.Context(), req.ID)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, info)
}

func (wh *webHandler) handleJobStop(w hRW, r *hR, _ hP) {
	req := struct {
		ID string `json:"id"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.JobStop(r.Context(), req.ID)
	wh.writeError(w, err)
}

// -----------------------------------------about session------------------------------------------

func (wh *webHandler) handleSessionList(w hRW, r *hR, _ hP) {
	list, err := wh.ctx.SessionList(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Sessions map[uint64]*SessionInfo `json:"sessions"`
	}{
		Sessions: list,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleSessionStop(w hRW, r *hR, _ hP) {
	req := struct {
		ID uint64 `json:"id"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	// first check is in web handler
	err = wh.ctx.SessionStop(r.Context(), req.ID)
	wh.writeError(w, err)
}

func (wh *webHandler) handleSessionShellRead(w hRW, r *hR, _ hP) {
	req := struct {
		ID uint64 `json:"id"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	result, err := wh.ctx.SessionShellRead(r.Context(), req.ID)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Data string `json:"data"`
	}{
		Data: result.Data,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleSessionShellWrite(w hRW, r *hR, _ hP) {
	req := struct {
		ID   uint64 `json:"id"`
		Data string `json:"data"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	// check
	_, err = wh.ctx.SessionShellWrite(r.Context(), req.ID, req.Data)
	wh.writeError(w, err)
}

func (wh *webHandler) handleSessionUpgrade(w hRW, r *hR, _ hP) {
	req := struct {
		ID      uint64                 `json:"id"`
		Host    string                 `json:"host"`
		Port    uint64                 `json:"port"`
		Options map[string]interface{} `json:"options"`
		Wait    time.Duration          `json:"wait"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	result, err := wh.ctx.SessionUpgrade(r.Context(),
		req.ID, req.Host, req.Port, req.Options, req.Wait)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		JobID uint64 `json:"job_id"`
	}{
		JobID: result.JobID,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleSessionMeterpreterRead(w hRW, r *hR, _ hP) {
	req := struct {
		ID uint64 `json:"id"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	data, err := wh.ctx.SessionMeterpreterRead(r.Context(), req.ID)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Data string `json:"data"`
	}{
		Data: data,
	}
	wh.writeResponse(w, &resp)
}

func (wh *webHandler) handleSessionMeterpreterWrite(w hRW, r *hR, _ hP) {
	req := struct {
		ID   uint64 `json:"id"`
		Data string `json:"data"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.SessionMeterpreterWrite(r.Context(), req.ID, req.Data)
	wh.writeError(w, err)
}

func (wh *webHandler) handleSessionMeterpreterSessionDetach(w hRW, r *hR, _ hP) {
	req := struct {
		ID uint64 `json:"id"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	// check exist
	err = wh.ctx.SessionMeterpreterSessionDetach(r.Context(), req.ID)
	wh.writeError(w, err)
}

func (wh *webHandler) handleSessionMeterpreterSessionKill(w hRW, r *hR, _ hP) {
	req := struct {
		ID uint64 `json:"id"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	// check exist
	err = wh.ctx.SessionMeterpreterSessionKill(r.Context(), req.ID)
	wh.writeError(w, err)
}

func (wh *webHandler) handleSessionMeterpreterRunSingle(w hRW, r *hR, _ hP) {
	req := struct {
		ID      uint64 `json:"id"`
		Command string `json:"command"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.SessionMeterpreterRunSingle(r.Context(), req.ID, req.Command)
	wh.writeError(w, err)
}

func (wh *webHandler) handleSessionCompatibleModules(w hRW, r *hR, _ hP) {
	req := struct {
		ID uint64 `json:"id"`
	}{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	modules, err := wh.ctx.SessionCompatibleModules(r.Context(), req.ID)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	resp := struct {
		Modules []string `json:"modules"`
	}{
		Modules: modules,
	}
	wh.writeResponse(w, &resp)
}
