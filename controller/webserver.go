package controller

import (
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/security"
	"project/internal/xpanic"
)

type hRW = http.ResponseWriter
type hR = http.Request
type hP = httprouter.Params

type web struct {
	ctx      *CTRL
	listener net.Listener
	server   *http.Server
	indexFS  http.Handler // index file system
}

func newWeb(ctx *CTRL, cfg *Config) (*web, error) {
	// listen tls
	certFile := cfg.HTTPSCertFile
	keyFile := cfg.HTTPSKeyFile
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	listener, err := net.Listen("tcp", cfg.HTTPSAddress)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// router
	web := &web{
		ctx:      ctx,
		listener: listener,
	}
	router := &httprouter.Router{
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		HandleOPTIONS:          true,
		PanicHandler:           web.handlePanic,
	}
	// resource
	router.ServeFiles("/css/*filepath", http.Dir(cfg.HTTPSWebDir+"/css"))
	router.ServeFiles("/js/*filepath", http.Dir(cfg.HTTPSWebDir+"/js"))
	router.ServeFiles("/img/*filepath", http.Dir(cfg.HTTPSWebDir+"/img"))
	web.indexFS = http.FileServer(http.Dir(cfg.HTTPSWebDir))
	handleFavicon := func(w hRW, r *hR, _ hP) {
		web.indexFS.ServeHTTP(w, r)
	}
	router.GET("/favicon.ico", handleFavicon)
	router.GET("/", web.handleIndex)
	router.GET("/login", web.handleLogin)
	router.POST("/load_keys", web.handleLoadKeys)
	// debug api
	router.GET("/api/debug/shutdown", web.handleShutdown)
	// operate
	router.GET("/api/boot", web.handleGetBoot)
	router.POST("/api/node/trust", web.handleTrustNode)
	// http server
	tlsConfig := &tls.Config{
		Certificates: make([]tls.Certificate, 1),
	}
	tlsConfig.Certificates[0] = cert
	web.server = &http.Server{
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: time.Minute,
		Handler:           router,
		ErrorLog:          logger.Wrap(logger.WARNING, "web", ctx),
	}
	return web, nil
}

func (web *web) Deploy() error {
	errChan := make(chan error, 1)
	serve := func() {
		errChan <- web.server.ServeTLS(web.listener, "", "")
		web.ctx.wg.Done()
	}
	web.ctx.wg.Add(1)
	go serve()
	select {
	case err := <-errChan:
		return errors.WithStack(err)
	case <-time.After(time.Second):
		return nil
	}
}

func (web *web) Address() string {
	return web.listener.Addr().String()
}

func (web *web) Close() {
	_ = web.server.Close()
}

func (web *web) handlePanic(w hRW, r *hR, e interface{}) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(xpanic.Sprint(e)))
}

func (web *web) handleLogin(w hRW, r *hR, p hP) {
	_, _ = w.Write([]byte("hello"))
}

func (web *web) handleLoadKeys(w hRW, r *hR, p hP) {
	pwd, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	err = web.ctx.LoadKeys(string(pwd))
	security.FlushBytes(pwd)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_, _ = w.Write([]byte("ok"))
}

func (web *web) handleIndex(w hRW, r *hR, p hP) {
	web.indexFS.ServeHTTP(w, r)
}
