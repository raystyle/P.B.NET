package controller

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/logger"
	"project/internal/security"
	"project/internal/xpanic"
)

type hRW = http.ResponseWriter
type hR = http.Request
type hP = httprouter.Params

type web struct {
	ctx *CTRL

	listener net.Listener
	server   *http.Server
	indexFS  http.Handler // index file system

	wg sync.WaitGroup
}

func newWeb(ctx *CTRL, config *Config) (*web, error) {
	cfg := config.Web
	// listen tls
	certFile := cfg.CertFile
	keyFile := cfg.KeyFile
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// router
	web := web{
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
	router.ServeFiles("/css/*filepath", http.Dir(cfg.Dir+"/css"))
	router.ServeFiles("/js/*filepath", http.Dir(cfg.Dir+"/js"))
	router.ServeFiles("/img/*filepath", http.Dir(cfg.Dir+"/img"))
	web.indexFS = http.FileServer(http.Dir(cfg.Dir))
	handleFavicon := func(w hRW, r *hR, _ hP) {
		web.indexFS.ServeHTTP(w, r)
	}
	router.GET("/favicon.ico", handleFavicon)
	router.GET("/", web.handleIndex)
	router.GET("/login", web.handleLogin)
	router.POST("/load_keys", web.handleLoadKeys)

	// debug api
	router.GET("/api/debug/shutdown", web.handleShutdown)

	// API
	router.GET("/api/boot", web.handleGetBoot)
	router.POST("/api/node/trust", web.handleTrustNode)

	// HTTPS server
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	web.server = &http.Server{
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: time.Minute,
		Handler:           router,
		ErrorLog:          logger.Wrap(logger.Warning, "web", ctx.logger),
	}
	return &web, nil
}

func (web *web) Deploy() error {
	errChan := make(chan error, 1)
	serve := func() {
		errChan <- web.server.ServeTLS(web.listener, "", "")
		web.wg.Done()
	}
	web.wg.Add(1)
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
	web.ctx = nil
}

func (web *web) handlePanic(w hRW, r *hR, e interface{}) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = io.Copy(w, xpanic.Print(e, "web"))
}

func (web *web) handleLogin(w hRW, r *hR, p hP) {
	_, _ = w.Write([]byte("hello"))
}

func (web *web) handleLoadKeys(w hRW, r *hR, p hP) {
	// TODO size
	pwd, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	err = web.ctx.LoadSessionKey(pwd)
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

// ------------------------------debug API----------------------------------

func (web *web) handleShutdown(w hRW, r *hR, p hP) {
	_ = r.ParseForm()
	errStr := r.FormValue("err")
	_, _ = w.Write([]byte("ok"))
	if errStr != "" {
		web.ctx.Exit(errors.New(errStr))
	} else {
		web.ctx.Exit(nil)
	}
}

// ---------------------------------API-------------------------------------

func (web *web) handleGetBoot(w hRW, r *hR, p hP) {
	_, _ = w.Write([]byte("hello"))
}

func (web *web) handleTrustNode(w hRW, r *hR, p hP) {
	m := &mTrustNode{}
	err := json.NewDecoder(r.Body).Decode(m)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	n := &bootstrap.Node{
		Mode:    m.Mode,
		Network: m.Network,
		Address: m.Address,
	}
	req, err := web.ctx.TrustNode(context.TODO(), n)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	b, err := json.Marshal(req)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_, _ = w.Write(b)
}
