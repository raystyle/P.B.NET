package controller

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"

	"project/internal/logger"
)

type h_rw = http.ResponseWriter
type h_r = http.Request
type h_p = httprouter.Params

type http_server struct {
	ctx      *CTRL
	listener net.Listener
	server   *http.Server
	index_fs http.Handler
}

func new_http_server(ctx *CTRL, c *Config) (*http_server, error) {
	// listen tls
	crt_path := "cert/server.crt"
	key_path := "cert/server.key"
	crt, err := tls.LoadX509KeyPair(crt_path, key_path)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	listener, err := net.Listen("tcp", c.HTTP_Address)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// router
	hs := &http_server{
		ctx:      ctx,
		listener: listener,
	}
	router := &httprouter.Router{
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		HandleOPTIONS:          true,
	}
	router.ServeFiles("/css/*filepath", http.Dir("web/css"))
	router.ServeFiles("/js/*filepath", http.Dir("web/js"))
	router.ServeFiles("/img/*filepath", http.Dir("web/img"))
	fs := http.FileServer(http.Dir("web"))
	handle_favicon := func(w h_rw, r *h_r, _ h_p) {
		fs.ServeHTTP(w, r)
	}
	router.GET("/favicon.ico", handle_favicon)
	hs.index_fs = fs
	router.GET("/", hs.h_index)
	router.GET("login", hs.h_login)
	router.GET("/bootstrapper", hs.h_get_bootstrapper)
	tls_config := &tls.Config{
		Certificates: make([]tls.Certificate, 1),
	}
	tls_config.Certificates[0] = crt
	hs.server = &http.Server{
		TLSConfig:         tls_config,
		ReadHeaderTimeout: time.Minute,
		Handler:           router,
		ErrorLog:          logger.Wrap(logger.WARNING, "httpserver", ctx),
	}
	return hs, nil
}

func (this *http_server) Serve() error {
	err_chan := make(chan error, 1)
	serve := func() {
		err_chan <- this.server.ServeTLS(this.listener, "", "")
		this.ctx.wg.Done()
	}
	this.ctx.wg.Add(1)
	go serve()
	select {
	case err := <-err_chan:
		return errors.WithStack(err)
	case <-time.After(time.Second):
		return nil
	}
}

func (this *http_server) Address() string {
	return this.listener.Addr().String()
}

func (this *http_server) Close() {
	_ = this.server.Close()
}

func (this *http_server) h_index(w h_rw, r *h_r, p h_p) {
	this.index_fs.ServeHTTP(w, r)
}
