package xpprof

import (
	"net"
	"net/http"
	"net/http/pprof"
	"sync"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/security"
)

// Server is an pprof tool with http server.
type Server struct {
	logger   logger.Logger
	https    bool
	maxConns int
	logSrc   string

	server  *http.Server
	handler *handler

	// listener addresses
	addresses    map[*net.Addr]struct{}
	addressesRWM sync.RWMutex
}

// NewHTTPServer is used to create a pprof tool with http server.
func NewHTTPServer(lg logger.Logger, opts *Options) (*Server, error) {
	return newServer(lg, opts, false)
}

// NewHTTPServer is used to create a pprof tool with http server.
func NewHTTPSServer(lg logger.Logger, opts *Options) (*Server, error) {
	return newServer(lg, opts, true)
}

func newServer(lg logger.Logger, opts *Options, https bool) (*Server, error) {
	if opts == nil {
		opts = new(Options)
	}
	srv := Server{
		logger:   lg,
		https:    https,
		maxConns: opts.MaxConns,
	}
	// apply options
	var err error
	srv.server, err = opts.Server.Apply()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	timeout := opts.Timeout
	if timeout < 1 {
		timeout = defaultTimeout
	}
	srv.server.ReadTimeout = timeout
	srv.server.WriteTimeout = timeout
	if srv.maxConns < 1 {
		srv.maxConns = defaultMaxConnections
	}
	// log source
	var logSrc string
	if srv.https {
		logSrc = "pprof-https"
	} else {
		logSrc = "pprof-http"
	}
	srv.logSrc = logSrc
}

// NewServerWithListener is used to create a pprof server with listener.
func NewServerWithListener(listener net.Listener, opts *Options) (*Server, error) {
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/debug/pprof/", pprof.Index)
	serveMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	serveMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	serveMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	serveMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	server := &http.Server{Handler: serveMux}

}

func (server *Server) ListenAndServe(network, address string) error {
	err := CheckNetwork(network)
	if err != nil {
		return err
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		return errors.WithStack(err)
	}
	return server.Serve(listener)
}

func (server *Server) Serve(listener net.Listener) (err error) {

}

type handler struct {
	username *security.Bytes
	password *security.Bytes
}
