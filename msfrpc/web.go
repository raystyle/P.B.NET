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

// subFileSystem is used to open sub directory for http file server.
type subFileSystem struct {
	fs   http.FileSystem
	path string
}

func newSubFileSystem(fs http.FileSystem, path string) *subFileSystem {
	return &subFileSystem{fs: fs, path: path + "/"}
}

func (sfs *subFileSystem) Open(name string) (http.File, error) {
	return sfs.fs.Open(sfs.path + name)
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

// WebServerOptions contains options about web server.
type WebServerOptions struct {
	option.HTTPServer
	MaxConns int

	// TODO max body size

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
	fs http.FileSystem,
	opts *WebServerOptions,
) (*WebServer, error) {

	httpServer, err := opts.HTTPServer.Apply()
	if err != nil {
		return nil, err
	}

	// configure handler.
	wh := webHandler{ctx: msf}
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
	router.ServeFiles("/css/*filepath", newSubFileSystem(fs, "css"))
	router.ServeFiles("/js/*filepath", newSubFileSystem(fs, "js"))
	router.ServeFiles("/img/*filepath", newSubFileSystem(fs, "img"))

	httpServer.Handler = router

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

func (wh *webHandler) handleAuthLogout(w hRW, r *hR, _ hP) {
	req := AuthLogoutRequest{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.AuthLogout(req.Token)
	wh.writeError(w, err)
}

func (wh *webHandler) handleAuthTokenList(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleAuthTokenGenerate(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleAuthTokenAdd(w hRW, r *hR, _ hP) {
	req := AuthTokenAddRequest{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.AuthTokenAdd(r.Context(), req.Token)
	wh.writeError(w, err)
}

func (wh *webHandler) handleAuthTokenRemove(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleCoreSetG(w hRW, r *hR, _ hP) {
	req := CoreSetGRequest{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.CoreSetG(r.Context(), req.Name, req.Value)
	wh.writeError(w, err)
}

func (wh *webHandler) handleCoreUnsetG(w hRW, r *hR, _ hP) {
	req := CoreUnsetGRequest{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.CoreUnsetG(r.Context(), req.Name)
	wh.writeError(w, err)
}

func (wh *webHandler) handleCoreGetG(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBStatus(w hRW, r *hR, _ hP) {
	status, err := wh.ctx.DBStatus(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, status)
}

func (wh *webHandler) handleDBReportHost(w hRW, r *hR, _ hP) {
	req := DBReportHost{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.DBReportHost(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDBHost(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBGetHost(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBDelHost(w hRW, r *hR, _ hP) {
	req := DBDelHostOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	_, err = wh.ctx.DBDelHost(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDBReportService(w hRW, r *hR, _ hP) {
	req := DBReportService{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.DBReportService(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDBService(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBGetService(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBDelService(w hRW, r *hR, _ hP) {
	req := DBDelServiceOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	_, err = wh.ctx.DBDelService(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDBReportClient(w hRW, r *hR, _ hP) {
	req := DBReportClient{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.DBReportClient(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDBClients(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBGetClient(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBDelClient(w hRW, r *hR, _ hP) {
	req := DBDelClientOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	_, err = wh.ctx.DBDelClient(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDBCreateCredential(w hRW, r *hR, _ hP) {
	req := DBCreateCredentialOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	_, err = wh.ctx.DBCreateCredential(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDBCredential(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBReportLoot(w hRW, r *hR, _ hP) {
	req := DBReportLoot{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.DBReportLoot(r.Context(), &req)
	wh.writeError(w, err)
}

func (wh *webHandler) handleDBLoots(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBWorkspaces(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBGetWorkspace(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBAddWorkspace(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBDelWorkspace(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBSetWorkspace(w hRW, r *hR, _ hP) {
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

func (wh *webHandler) handleDBCurrentWorkspace(w hRW, r *hR, _ hP) {
	result, err := wh.ctx.DBCurrentWorkspace(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, result)
}

func (wh *webHandler) handleDBEvent(w hRW, r *hR, _ hP) {
	req := DBEventOptions{}
	err := wh.readRequest(r, &req)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	// err = wh.ctx.DBEvent(r.Context(), &req)
	// wh.writeError(w, err)
}

func (wh *webHandler) handleDBImportData(w hRW, r *hR, _ hP) {

}

func (wh *webHandler) handle(w hRW, r *hR, _ hP) {

}
