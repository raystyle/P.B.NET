package msfrpc

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/bcrypt"

	"project/internal/logger"
	"project/internal/option"
	"project/internal/patch/json"
	"project/internal/virtualconn"
	"project/internal/xpanic"
)

type hRW = http.ResponseWriter
type hR = http.Request
type hP = httprouter.Params

// WebServerOptions contains options about web server.
type WebServerOptions struct {
	option.HTTPServer
	MaxConns int    `toml:"max_conns"`
	Username string `toml:"username"`
	Password string `toml:"password"`
	// about Console, Shell and Meterpreter IO interval
	IOInterval      time.Duration `toml:"io_interval"`
	MonitorInterval time.Duration `toml:"monitor_interval"`
}

// WebServer is provide a web UI.
type WebServer struct {
	server  *http.Server
	handler *webHandler

	wg sync.WaitGroup
}

// NewWebServer is used to create a web server.
func (msf *MSFRPC) NewWebServer(
	listener net.Listener,
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
	router.ServeFiles("/css/*filepath", fs)
	router.ServeFiles("/js/*filepath", fs)
	router.ServeFiles("/img/*filepath", fs)

	httpServer.Handler = router

	web := WebServer{
		server:  httpServer,
		handler: &wh,
	}
	web.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// TODO panic
			}
			web.wg.Done()
		}()
		switch listener.(type) {
		case *virtualconn.Listener:
			err := httpServer.Serve(listener)
			fmt.Println(err)
		default:
			err := httpServer.ServeTLS(listener, "", "")
			fmt.Println(err)
		}
	}()
	return &web, nil
}

// Close is used to close web server.
func (web *WebServer) Close() {
	_ = web.server.Close()

}

type webHandler struct {
	ctx *MSFRPC

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
	hash, err := bcrypt.GenerateFromPassword([]byte{1, 2, 3}, 15)
	fmt.Println(string(hash), err)
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
