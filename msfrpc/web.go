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

func (web *WebServer) onEvent(event string) {

}

// shortcut about interface and structure.
type hRW = http.ResponseWriter
type hR = http.Request
type hP = httprouter.Params

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

type webError struct {
	Error string `json:"error"`
}

func (wh *webHandler) writeError(w hRW, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	e := webError{}
	if err != nil {
		e.Error = err.Error()
	}
	encoder := wh.encoderPool.Get().(*json.Encoder)
	defer wh.encoderPool.Put(encoder)
	data, err := encoder.Encode(e)
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(data)
}

func (wh *webHandler) writeResponse(w hRW, response interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	encoder := wh.encoderPool.Get().(*json.Encoder)
	defer wh.encoderPool.Put(encoder)
	data, err := encoder.Encode(response)
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(data)
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

func (wh *webHandler) handleTokenList(w hRW, r *hR, _ hP) {
	tokens, err := wh.ctx.AuthTokenList(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, tokens)
}

func (wh *webHandler) handleTokenGenerate(w hRW, r *hR, _ hP) {
	token, err := wh.ctx.AuthTokenGenerate(r.Context())
	if err != nil {
		wh.writeError(w, err)
		return
	}
	s := struct {
		Token string `json:"token"`
	}{
		Token: token,
	}
	wh.writeResponse(w, &s)
}
