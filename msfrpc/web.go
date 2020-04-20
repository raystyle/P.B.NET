package msfrpc

import (
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

// shortcut about interface
type hRW = http.ResponseWriter
type hR = http.Request
type hP = httprouter.Params

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

// WebServerOptions contains options about web server.
type WebServerOptions struct {
	option.HTTPServer
	MaxConns int
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
		OnToken:      web.onToken,
		OnJob:        web.onJob,
		OnSession:    web.onSession,
		OnHost:       web.onHost,
		OnCredential: web.onCredential,
		OnLoot:       web.onLoot,
		OnEvent:      web.onEvent,
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

// callbacks to notice web UI that some data is updated.

func (web *WebServer) onToken(token string, add bool) {

}

func (web *WebServer) onJob(id, name string, active bool) {

}

func (web *WebServer) onSession(id uint64, info *SessionInfo, opened bool) {

}

func (web *WebServer) onHost(workspace string, host *DBHost, add bool) {

}

func (web *WebServer) onCredential(workspace string, cred *DBCred, add bool) {

}

func (web *WebServer) onLoot(workspace string, loot *DBLoot) {

}

func (web *WebServer) onEvent(workspace string, event *DBEvent) {

}

type webHandler struct {
	ctx *MSFRPC

	username   string
	password   string
	ioInterval time.Duration

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
