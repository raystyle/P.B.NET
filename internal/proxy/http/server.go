package http

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/httptool"
	"project/internal/logger"
	"project/internal/xpanic"
)

// Server implemented internal/proxy.server.
type Server struct {
	tag      string
	logger   logger.Logger
	https    bool
	maxConns int

	// accept client request
	server  *http.Server
	handler *handler

	// listener addresses
	addresses    map[*net.Addr]struct{}
	addressesRWM sync.RWMutex

	closeOnce sync.Once
}

// NewHTTPServer is used to create a HTTPS proxy server.
func NewHTTPServer(tag string, lg logger.Logger, opts *Options) (*Server, error) {
	return newServer(tag, lg, opts, false)
}

// NewHTTPSServer is used to create a HTTPS proxy server.
func NewHTTPSServer(tag string, lg logger.Logger, opts *Options) (*Server, error) {
	return newServer(tag, lg, opts, true)
}

func newServer(tag string, lg logger.Logger, opts *Options, https bool) (*Server, error) {
	if tag == "" {
		return nil, errors.New("empty tag")
	}
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
	transport, err := opts.Transport.Apply()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	timeout := opts.Timeout
	if timeout < 1 {
		timeout = defaultConnectTimeout
	}
	srv.server.ReadTimeout = timeout
	srv.server.WriteTimeout = timeout
	if srv.maxConns < 1 {
		srv.maxConns = defaultMaxConnections
	}
	// tag
	if srv.https {
		tag = "https proxy-" + tag
	} else {
		tag = "http proxy-" + tag
	}
	srv.tag = tag
	// initialize http handler
	handler := &handler{
		tag:         tag,
		logger:      lg,
		timeout:     timeout,
		transport:   transport,
		dialContext: opts.DialContext,
	}
	if handler.dialContext == nil {
		handler.dialContext = new(net.Dialer).DialContext
	}
	if opts.DialContext != nil {
		transport.DialContext = opts.DialContext
	}
	if opts.Username != "" {
		handler.username = []byte(opts.Username)
	}
	if opts.Password != "" {
		handler.password = []byte(opts.Password)
	}
	handler.ctx, handler.cancel = context.WithCancel(context.Background())

	srv.handler = handler
	srv.server.Handler = handler
	srv.server.ErrorLog = logger.Wrap(logger.Error, srv.tag, lg)
	srv.addresses = make(map[*net.Addr]struct{})
	return &srv, nil
}

func (s *Server) logf(lv logger.Level, format string, log ...interface{}) {
	s.logger.Printf(lv, s.tag, format, log...)
}

func (s *Server) log(lv logger.Level, log ...interface{}) {
	s.logger.Println(lv, s.tag, log...)
}

func (s *Server) addAddress(addr *net.Addr) {
	s.addressesRWM.Lock()
	defer s.addressesRWM.Unlock()
	s.addresses[addr] = struct{}{}
}

func (s *Server) deleteAddress(addr *net.Addr) {
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
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "Server.Serve")
			s.log(logger.Fatal, err)
		}
	}()

	listener = netutil.LimitListener(listener, s.maxConns)
	address := listener.Addr()
	network := address.Network()
	s.addAddress(&address)
	defer s.deleteAddress(&address)

	s.logf(logger.Info, "start listener (%s %s)", network, address)
	defer s.logf(logger.Info, "listener closed (%s %s)", network, address)

	if s.https {
		err = s.server.ServeTLS(listener, "", "")
	} else {
		err = s.server.Serve(listener)
	}
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Addresses is used to get listener addresses.
func (s *Server) Addresses() []net.Addr {
	s.addressesRWM.RLock()
	defer s.addressesRWM.RUnlock()
	addresses := make([]net.Addr, 0, len(s.addresses))
	for address := range s.addresses {
		addresses = append(addresses, *address)
	}
	return addresses
}

// Info is used to get http proxy server information.
//
// "address: tcp 127.0.0.1:1999, tcp4 127.0.0.1:2001"
// "address: tcp 127.0.0.1:1999 auth: admin:123456"
func (s *Server) Info() string {
	buf := new(bytes.Buffer)
	addresses := s.Addresses()
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
	username := s.handler.username
	password := s.handler.password
	if username != nil || password != nil {
		format := "auth: %s:%s"
		if buf.Len() > 0 {
			format = " " + format
		}
		_, _ = fmt.Fprintf(buf, format, username, password)
	}
	return buf.String()
}

// Close is used to close HTTP proxy server.
func (s *Server) Close() error {
	var err error
	s.closeOnce.Do(func() {
		err = s.server.Close()
		s.handler.Close()
	})
	return err
}

var (
	connectionEstablished    = []byte("HTTP/1.0 200 Connection established\r\n\r\n")
	connectionEstablishedLen = len(connectionEstablished)
)

type handler struct {
	tag    string
	logger logger.Logger

	// dial timeout
	timeout time.Duration

	// proxy server http client
	transport *http.Transport

	// secondary proxy
	dialContext func(ctx context.Context, network, address string) (net.Conn, error)

	username []byte
	password []byte

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// [2018-11-27 00:00:00] [info] <http proxy-tag> test log
// client: 127.0.0.1:1234
// POST /index HTTP/1.1
// Host: github.com
// Accept: text/html
// Connection: keep-alive
// User-Agent: Mozilla
//
// post data...
// post data...
func (h *handler) log(lv logger.Level, r *http.Request, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintln(buf, log...)
	_, _ = httptool.FprintRequest(buf, r)
	h.logger.Println(lv, h.tag, buf)
}

// ServeHTTP implement http.Handler.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.wg.Add(1)
	defer h.wg.Done()

	const title = "server.ServeHTTP()"
	defer func() {
		if rec := recover(); rec != nil {
			h.log(logger.Fatal, r, xpanic.Print(rec, title))
		}
	}()

	if !h.authenticate(w, r) {
		return
	}
	h.log(logger.Info, r, "handle request")
	// remove Proxy-Authorization
	r.Header.Del("Proxy-Authorization")
	if r.Method == http.MethodConnect { // handle https
		// hijack client conn
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			panic("http.ResponseWriter don't implemented http.Hijacker")
		}
		wc, _, err := hijacker.Hijack()
		if err != nil {
			h.log(logger.Error, r, err)
			return
		}
		defer func() { _ = wc.Close() }()

		// dial target
		ctx, cancel := context.WithTimeout(h.ctx, h.timeout)
		defer cancel()
		conn, err := h.dialContext(ctx, "tcp", r.URL.Host)
		if err != nil {
			h.log(logger.Error, r, err)
			return
		}
		defer func() { _ = conn.Close() }()
		_, err = wc.Write(connectionEstablished)
		if err != nil {
			h.log(logger.Error, r, "failed to write response:", err)
			return
		}
		// http.Server.Close() not close hijacked conn
		closeChan := make(chan struct{})
		h.wg.Add(1)
		go func() {
			defer h.wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					h.log(logger.Fatal, r, xpanic.Print(rec, title))
				}
			}()
			select {
			case <-closeChan:
			case <-h.ctx.Done():
				_ = wc.Close()
				_ = conn.Close()
			}
		}()
		// start copy
		h.wg.Add(1)
		go func() {
			defer h.wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					h.log(logger.Fatal, r, xpanic.Print(rec, title))
				}
			}()
			_, _ = io.Copy(wc, conn)
		}()
		_, _ = io.Copy(conn, wc)
		close(closeChan)
	} else { // handle http request
		ctx, cancel := context.WithTimeout(h.ctx, h.timeout)
		defer cancel()
		resp, err := h.transport.RoundTrip(r.Clone(ctx))
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			h.log(logger.Error, r, err)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		// copy header
		for k, v := range resp.Header {
			w.Header().Set(k, v[0])
		}
		// write status and body
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}

func (h *handler) authenticate(w http.ResponseWriter, r *http.Request) bool {
	if len(h.username) == 0 && len(h.password) == 0 {
		return true
	}
	failedToAuth := func() {
		w.Header().Set("Proxy-Authenticate", "Basic")
		w.WriteHeader(http.StatusProxyAuthRequired)
	}
	authInfo := strings.Split(r.Header.Get("Proxy-Authorization"), " ")
	if len(authInfo) != 2 {
		failedToAuth()
		return false
	}
	authMethod := authInfo[0]
	authBase64 := authInfo[1]
	switch authMethod {
	case "Basic":
		auth, err := base64.StdEncoding.DecodeString(authBase64)
		if err != nil {
			h.log(logger.Exploit, r, "invalid basic base64 data:", err)
			failedToAuth()
			return false
		}
		userPass := strings.Split(string(auth), ":")
		if len(userPass) < 2 {
			userPass = append(userPass, "")
		}
		user := []byte(userPass[0])
		pass := []byte(userPass[1])
		if subtle.ConstantTimeCompare(h.username, user) != 1 ||
			subtle.ConstantTimeCompare(h.password, pass) != 1 {
			userInfo := fmt.Sprintf("%s:%s", user, pass)
			h.log(logger.Exploit, r, "invalid username or password:", userInfo)
			failedToAuth()
			return false
		}
		return true
	default:
		h.log(logger.Exploit, r, "unsupported authenticate method:", authMethod)
		failedToAuth()
		return false
	}
}

func (h *handler) Close() {
	h.cancel()
	h.wg.Wait()
	h.transport.CloseIdleConnections()
}
