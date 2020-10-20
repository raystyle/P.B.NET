package xpprof

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/httptool"
	"project/internal/logger"
	"project/internal/nettool"
	"project/internal/security"
	"project/internal/xpanic"
	"project/internal/xsync"
)

// Server is a pprof tool with http server.
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

// NewHTTPSServer is used to create a pprof tool with https server.
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
	// initialize http handler
	handler := &handler{
		logger: lg,
		logSrc: logSrc,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	handler.mux = mux
	if opts.Username != "" {
		handler.username = security.NewBytes([]byte(opts.Username))
	}
	if opts.Password != "" {
		handler.password = security.NewBytes([]byte(opts.Password))
	}
	// pprof http server
	srv.handler = handler
	srv.server.Handler = mux
	srv.server.ErrorLog = logger.Wrap(logger.Error, logSrc, lg)
	srv.server.ConnState = func(conn net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			handler.counter.Add(1)
		case http.StateHijacked, http.StateClosed:
			handler.counter.Done()
		}
	}
	srv.addresses = make(map[*net.Addr]struct{})
	return &srv, nil
}

func (s *Server) logf(lv logger.Level, format string, log ...interface{}) {
	s.logger.Printf(lv, s.logSrc, format, log...)
}

func (s *Server) log(lv logger.Level, log ...interface{}) {
	s.logger.Println(lv, s.logSrc, log...)
}

func (s *Server) addListenerAddress(addr *net.Addr) {
	s.addressesRWM.Lock()
	defer s.addressesRWM.Unlock()
	s.addresses[addr] = struct{}{}
}

func (s *Server) deleteListenerAddress(addr *net.Addr) {
	s.addressesRWM.Lock()
	defer s.addressesRWM.Unlock()
	delete(s.addresses, addr)
}

// ListenAndServe is used to listen a listener and serve.
func (s *Server) ListenAndServe(network, address string) error {
	err := CheckNetwork(network)
	if err != nil {
		return err
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		return errors.WithStack(err)
	}
	return s.Serve(listener)
}

// Serve accepts incoming connections on the listener.
func (s *Server) Serve(listener net.Listener) (err error) {
	s.handler.counter.Add(1)
	defer s.handler.counter.Done()

	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "Server.Serve")
			s.log(logger.Fatal, err)
		}
	}()

	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "Server.Serve")
			s.log(logger.Fatal, err)
		}
	}()

	listener = netutil.LimitListener(listener, s.maxConns)
	defer func() { _ = listener.Close() }()

	address := listener.Addr()
	network := address.Network()
	s.addListenerAddress(&address)
	defer s.deleteListenerAddress(&address)

	s.logf(logger.Info, "start listener (%s %s)", network, address)
	defer s.logf(logger.Info, "listener closed (%s %s)", network, address)

	if s.https {
		err = s.server.ServeTLS(listener, "", "")
	} else {
		err = s.server.Serve(listener)
	}

	if nettool.IsNetClosingError(err) || err == http.ErrServerClosed {
		return nil
	}
	return err
}

type handler struct {
	logger logger.Logger
	logSrc string

	mux *http.ServeMux

	username *security.Bytes
	password *security.Bytes

	counter xsync.Counter
}

// [2018-11-27 00:00:00] [info] <pprof-http> test log
// remote: 127.0.0.1:1234
// POST /index HTTP/1.1
// Host: github.com
// Accept: text/html
// Connection: keep-alive
// User-Agent: Mozilla
func (h *handler) log(lv logger.Level, r *http.Request, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintln(buf, log...)
	_, _ = httptool.FprintRequest(buf, r)
	h.logger.Println(lv, h.logSrc, buf)
}

// ServeHTTP implement http.Handler.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			h.log(logger.Fatal, r, xpanic.Print(rec, "server.ServeHTTP()"))
		}
	}()
	// authenticate

	h.mux.ServeHTTP(w, r)
}
