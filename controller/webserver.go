package controller

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

type http_server struct {
	ctx      *CTRL
	listener net.Listener
	server   *http.Server
}

func new_http_server(ctx *CTRL, c *Config) (*http_server, error) {
	// listen tls
	crt, err := tls.LoadX509KeyPair("cert/server.crt", "cert/server.key")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	listener, err := net.Listen("tcp", c.HTTP_Address)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// router
	router := &httprouter.Router{
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		HandleOPTIONS:          true,
	}
	router.GET("/bootstrapper", get_bootstrapper)
	// http server
	tls_config := &tls.Config{
		Certificates: make([]tls.Certificate, 1),
	}
	tls_config.Certificates[0] = crt
	server := &http.Server{
		TLSConfig:         tls_config,
		ReadHeaderTimeout: time.Minute,
		Handler:           router,
	}
	hs := &http_server{
		ctx:      ctx,
		listener: listener,
		server:   server,
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
		return err
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
