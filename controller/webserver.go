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
		PanicHandler:           web.hPanic,
	}
	// resource
	router.ServeFiles("/css/*filepath", http.Dir(cfg.WebDir+"/css"))
	router.ServeFiles("/js/*filepath", http.Dir(cfg.WebDir+"/js"))
	router.ServeFiles("/img/*filepath", http.Dir(cfg.WebDir+"/img"))
	web.indexFS = http.FileServer(http.Dir(cfg.WebDir))
	hFavicon := func(w hRW, r *hR, _ hP) {
		web.indexFS.ServeHTTP(w, r)
	}
	router.GET("/favicon.ico", hFavicon)
	router.GET("/", web.hIndex)
	router.GET("/login", web.hLogin)
	router.POST("/load_keys", web.hLoadKeys)
	// debug api
	router.GET("/api/debug/shutdown", web.hShutdown)
	// operate
	router.GET("/api/boot", web.hGetBoot)
	router.POST("/api/node/trust", web.hTrustNode)
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

func (web *web) hPanic(w hRW, r *hR, e interface{}) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(xpanic.Sprint(e)))
}

func (web *web) hLogin(w hRW, r *hR, p hP) {
	_, _ = w.Write([]byte("hello"))
}

func (web *web) hLoadKeys(w hRW, r *hR, p hP) {
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

func (web *web) hIndex(w hRW, r *hR, p hP) {
	web.indexFS.ServeHTTP(w, r)
}
