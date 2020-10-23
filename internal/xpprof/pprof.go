package xpprof

import (
	"bytes"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/httptool"
	"project/internal/logger"
	"project/internal/nettool"
	"project/internal/option"
	"project/internal/security"
	"project/internal/xpanic"
	"project/internal/xsync"
)

const (
	defaultTimeout        = 15 * time.Second
	defaultMaxConnections = 1000
)

// Options contains options about pprof http server.
type Options struct {
	Username string            `toml:"username"`
	Password string            `toml:"password"`
	Timeout  time.Duration     `toml:"timeout"`
	MaxConns int               `toml:"max_conns"`
	Server   option.HTTPServer `toml:"server" check:"-"`
}

// Server is a pprof tool over http server.
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

// NewHTTPServer is used to create a pprof tool over http server.
func NewHTTPServer(lg logger.Logger, opts *Options) (*Server, error) {
	return newServer(lg, opts, false)
}

// NewHTTPSServer is used to create a pprof tool over https server.
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
	if opts.Username != "" { // escape
		username := url.User(opts.Username).String()
		handler.username = security.NewBytes([]byte(username))
	}
	if opts.Password != "" { // escape
		password := url.User(opts.Password).String()
		handler.password = security.NewBytes([]byte(password))
	}
	// pprof http server
	srv.handler = handler
	srv.server.Handler = handler
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

func (srv *Server) logf(lv logger.Level, format string, log ...interface{}) {
	srv.logger.Printf(lv, srv.logSrc, format, log...)
}

func (srv *Server) log(lv logger.Level, log ...interface{}) {
	srv.logger.Println(lv, srv.logSrc, log...)
}

func (srv *Server) addListenerAddress(addr *net.Addr) {
	srv.addressesRWM.Lock()
	defer srv.addressesRWM.Unlock()
	srv.addresses[addr] = struct{}{}
}

func (srv *Server) deleteListenerAddress(addr *net.Addr) {
	srv.addressesRWM.Lock()
	defer srv.addressesRWM.Unlock()
	delete(srv.addresses, addr)
}

// ListenAndServe is used to listen a listener and serve.
func (srv *Server) ListenAndServe(network, address string) error {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return errors.Errorf("unsupported network: %s", network)
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		return errors.WithStack(err)
	}
	return srv.Serve(listener)
}

// Serve accepts incoming connections on the listener.
func (srv *Server) Serve(listener net.Listener) (err error) {
	srv.handler.counter.Add(1)
	defer srv.handler.counter.Done()

	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "Server.Serve")
			srv.log(logger.Fatal, err)
		}
	}()

	listener = netutil.LimitListener(listener, srv.maxConns)
	defer func() { _ = listener.Close() }()

	address := listener.Addr()
	network := address.Network()
	srv.addListenerAddress(&address)
	defer srv.deleteListenerAddress(&address)

	srv.logf(logger.Info, "serve over listener (%s %s)", network, address)
	defer srv.logf(logger.Info, "listener closed (%s %s)", network, address)

	if srv.https {
		err = srv.server.ServeTLS(listener, "", "")
	} else {
		err = srv.server.Serve(listener)
	}

	if nettool.IsNetClosingError(err) || err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Addresses is used to get listener addresses.
func (srv *Server) Addresses() []net.Addr {
	srv.addressesRWM.RLock()
	defer srv.addressesRWM.RUnlock()
	addresses := make([]net.Addr, 0, len(srv.addresses))
	for address := range srv.addresses {
		addresses = append(addresses, *address)
	}
	return addresses
}

// Info is used to get pprof http server information.
//
// "address: tcp 127.0.0.1:1999, tcp4 127.0.0.1:2001"
// "address: tcp 127.0.0.1:1999 auth: admin:123456"
func (srv *Server) Info() string {
	buf := new(bytes.Buffer)
	addresses := srv.Addresses()
	l := len(addresses)
	if l > 0 {
		buf.WriteString("address: ")
		for i := 0; i < l; i++ {
			if i > 0 {
				buf.WriteString(", ")
			}
			network := addresses[i].Network()
			address := addresses[i].String()
			_, _ = fmt.Fprintf(buf, "%s %s", network, address)
		}
	}
	username := srv.handler.username
	password := srv.handler.password
	var (
		user string
		pass string
	)
	if username != nil {
		user = username.String()
	}
	if password != nil {
		pass = password.String()
	}
	if user != "" || pass != "" {
		format := "auth: %s:%s"
		if buf.Len() > 0 {
			format = " " + format
		}
		_, _ = fmt.Fprintf(buf, format, user, pass)
	}
	return buf.String()
}

// Close is used to close pprof http server.
func (srv *Server) Close() error {
	err := srv.server.Close()
	srv.handler.Close()
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
	if !h.authenticate(w, r) {
		return
	}
	h.log(logger.Info, r, "handle request")
	h.mux.ServeHTTP(w, r)
}

func (h *handler) authenticate(w http.ResponseWriter, r *http.Request) bool {
	if h.username == nil && h.password == nil {
		return true
	}
	authInfo := strings.Split(r.Header.Get("Authorization"), " ")
	if len(authInfo) != 2 {
		h.failedToAuth(w)
		return false
	}
	authMethod := authInfo[0]
	authBase64 := authInfo[1]
	switch authMethod {
	case "Basic":
		auth, err := base64.StdEncoding.DecodeString(authBase64)
		if err != nil {
			h.log(logger.Exploit, r, "invalid basic base64 data:", err)
			h.failedToAuth(w)
			return false
		}
		userPass := strings.Split(string(auth), ":")
		if len(userPass) < 2 {
			userPass = append(userPass, "")
		}
		user := []byte(userPass[0])
		pass := []byte(userPass[1])
		var (
			eUser []byte
			ePass []byte
		)
		if h.username != nil {
			eUser = h.username.Get()
			defer h.username.Put(eUser)
		}
		if h.password != nil {
			ePass = h.password.Get()
			defer h.password.Put(ePass)
		}
		userErr := subtle.ConstantTimeCompare(eUser, user) != 1
		passErr := subtle.ConstantTimeCompare(ePass, pass) != 1
		if userErr || passErr {
			auth := fmt.Sprintf("%s:%s", user, pass)
			h.log(logger.Exploit, r, "invalid username or password:", auth)
			h.failedToAuth(w)
			return false
		}
		return true
	default:
		h.log(logger.Exploit, r, "unsupported authenticate method:", authMethod)
		h.failedToAuth(w)
		return false
	}
}

func (h *handler) failedToAuth(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Basic")
	w.WriteHeader(http.StatusUnauthorized)
}

func (h *handler) Close() {
	h.counter.Wait()
}
