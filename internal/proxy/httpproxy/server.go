package httpproxy

import (
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

	"project/internal/logger"
	"project/internal/options"
)

var (
	connectionEstablished = []byte("HTTP/1.1 200 Connection established\r\n\r\n")
)

type Options struct {
	Username  string
	Password  string
	Server    *options.HTTPServer
	Transport *options.HTTPTransport
	MaxConns  int
}

type Server struct {
	tag       string
	logger    logger.Logger
	server    *http.Server
	transport *http.Transport // for client
	maxConns  int
	addr      string
	basicAuth []byte
	m         sync.Mutex
}

func NewServer(tag string, l logger.Logger, opts *Options) (*Server, error) {
	if tag == "" {
		return nil, errors.New("no tag")
	}
	if opts == nil {
		opts = new(Options)
	}
	s := &Server{
		tag:      tag,
		logger:   l,
		maxConns: opts.MaxConns,
	}
	var err error
	// http server
	if opts.Server != nil {
		s.server, err = opts.Server.Apply()
		if err != nil {
			return nil, errors.WithStack(err)
		}
	} else {
		s.server, _ = new(options.HTTPServer).Apply()
	}
	s.server.Handler = s
	s.server.ErrorLog = logger.Wrap(logger.Error, s.tag, l)
	// client transport
	if opts.Transport != nil {
		s.transport, err = opts.Transport.Apply()
		if err != nil {
			return nil, errors.WithStack(err)
		}
	} else {
		s.transport, _ = new(options.HTTPTransport).Apply()
	}
	if opts.MaxConns < 1 {
		s.maxConns = options.DefaultConnectionLimit
	}
	// basic authentication
	if opts.Username != "" {
		s.basicAuth = []byte(base64.StdEncoding.EncodeToString(
			[]byte(opts.Username + ":" + opts.Password)))
	}
	return s, nil
}

// from GOROOT/src/net/http/server.go
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	_ = tc.SetKeepAlive(true)
	_ = tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

func (s *Server) ListenAndServe(address string, timeout time.Duration) error {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	return s.Serve(tcpKeepAliveListener{l.(*net.TCPListener)}, timeout)
}

func (s *Server) Serve(l net.Listener, timeout time.Duration) error {
	s.m.Lock()
	defer s.m.Unlock()
	s.addr = l.Addr().String()
	limitListener := netutil.LimitListener(l, s.maxConns)
	return s.start(func() error { return s.server.Serve(limitListener) }, timeout)
}

func (s *Server) start(f func() error, timeout time.Duration) error {
	if timeout < 1 {
		timeout = options.DefaultStartTimeout
	}
	errChan := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			if r := recover(); r != nil {
				switch v := r.(type) {
				case error:
					err = v
				default:
					err = errors.New("unknown panic")
				}
			}
			errChan <- err
			close(errChan)
		}()
		err = f()
	}()
	select {
	case err := <-errChan:
		s.log(logger.Info, "start server failed:", err)
		return err
	case <-time.After(timeout):
		s.log(logger.Info, "start server success:", s.addr)
		return nil
	}
}

func (s *Server) Stop() error {
	err := s.server.Close()
	s.transport.CloseIdleConnections()
	s.log(logger.Info, "server stopped")
	return err
}

func (s *Server) Info() string {
	s.m.Lock()
	defer s.m.Unlock()
	auth, _ := base64.StdEncoding.DecodeString(string(s.basicAuth))
	return fmt.Sprintf("Listen: %s Auth: %s", s.addr, auth)
}

func (s *Server) Addr() string {
	s.m.Lock()
	defer s.m.Unlock()
	return s.addr
}

func (s *Server) log(l logger.Level, log ...interface{}) {
	s.logger.Println(l, s.tag, log...)
}

type log struct {
	Log interface{}
	Req *http.Request
}

func (l *log) String() string {
	return fmt.Sprint(l.Log, "\n", logger.HTTPRequest(l.Req))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			s.log(logger.Error, &log{Log: fmt.Sprint("panic: ", rec), Req: r})
		}
	}()
	// auth
	if s.basicAuth != nil {
		authFailed := func() {
			w.Header().Set("Proxy-Authenticate", "Basic")
			w.WriteHeader(http.StatusProxyAuthRequired)
		}
		auth := strings.Split(r.Header.Get("Proxy-Authorization"), " ")
		if len(auth) != 2 {
			// not log
			authFailed()
			return
		}
		authMethod := auth[0]
		authBase64 := auth[1]
		switch authMethod {
		case "Basic":
			if subtle.ConstantTimeCompare(s.basicAuth, []byte(authBase64)) != 1 {
				authFailed()
				s.log(logger.Exploit, &log{Log: "invalid basic authenticate", Req: r})
				return
			}
		default: // not support method
			authFailed()
			s.log(logger.Exploit, &log{Log: "unsupport auth method: " + authMethod, Req: r})
			return
		}
	}
	// handle https
	if r.Method == http.MethodConnect {
		// dial
		conn, err := s.transport.DialContext(context.Background(), "tcp", r.URL.Host)
		if err != nil {
			s.log(logger.Error, &log{Log: err, Req: r})
			return
		}
		// get client conn
		wc, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			s.log(logger.Error, &log{Log: err, Req: r})
			return
		}
		_, err = wc.Write(connectionEstablished)
		if err != nil {
			s.log(logger.Error, &log{Log: err, Req: r})
			return
		}
		go func() { _, _ = io.Copy(conn, wc) }()
		_, _ = io.Copy(wc, conn)
		_ = conn.Close()
		_ = wc.Close()
	} else { // handle http
		resp, err := s.transport.RoundTrip(r)
		if err != nil {
			s.log(logger.Error, &log{Log: err, Req: r})
			return
		}
		defer func() { _ = resp.Body.Close() }()
		// header
		for k, v := range resp.Header {
			w.Header().Set(k, v[0])
		}
		// write status and body
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}
