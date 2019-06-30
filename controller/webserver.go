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

type web struct {
	ctx      *CTRL
	listener net.Listener
	server   *http.Server
	index_fs http.Handler
}

func new_web(ctx *CTRL, c *Config) (*web, error) {
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
	hs := &web{
		ctx:      ctx,
		listener: listener,
	}
	router := &httprouter.Router{
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		HandleOPTIONS:          true,
	}
	// resource
	router.ServeFiles("/css/*filepath", http.Dir("web/css"))
	router.ServeFiles("/js/*filepath", http.Dir("web/js"))
	router.ServeFiles("/img/*filepath", http.Dir("web/img"))
	fs := http.FileServer(http.Dir("web"))
	hs.index_fs = fs
	handle_favicon := func(w h_rw, r *h_r, _ h_p) {
		fs.ServeHTTP(w, r)
	}
	router.GET("/favicon.ico", handle_favicon)
	router.GET("/", hs.h_index)
	router.GET("/login", hs.h_login)
	// api
	router.POST("/api/trust_node", hs.h_trust_node)
	router.GET("/api/bootstrapper", hs.h_get_bootstrapper)
	// http server
	tls_config := &tls.Config{
		Certificates: make([]tls.Certificate, 1),
	}
	tls_config.Certificates[0] = crt
	hs.server = &http.Server{
		TLSConfig:         tls_config,
		ReadHeaderTimeout: time.Minute,
		Handler:           router,
		ErrorLog:          logger.Wrap(logger.WARNING, "web", ctx),
	}
	return hs, nil
}

func (this *web) Deploy() error {
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

func (this *web) Address() string {
	return this.listener.Addr().String()
}

func (this *web) Close() {
	_ = this.server.Close()
}

func (this *web) h_login(w h_rw, r *h_r, p h_p) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hello"))
}

func (this *web) h_index(w h_rw, r *h_r, p h_p) {
	this.index_fs.ServeHTTP(w, r)
}
