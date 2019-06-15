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
	connection_established = []byte("HTTP/1.1 200 Connection established\r\n\r\n")
)

type Options struct {
	Username  string
	Password  string
	Server    *options.HTTP_Server
	Transport *options.HTTP_Transport
	Limit     int
}

type Server struct {
	tag        string
	logger     logger.Logger
	server     *http.Server
	transport  *http.Transport // for client
	limit      int
	addr       string
	basic_auth []byte
	m          sync.Mutex
}

func New_Server(tag string, l logger.Logger, opts *Options) (*Server, error) {
	if tag == "" {
		return nil, errors.New("no tag")
	}
	if opts == nil {
		opts = new(Options)
	}
	s := &Server{
		tag:    tag,
		logger: l,
		limit:  options.DEFAULT_CONNECTION_LIMIT,
	}
	var err error
	// http server
	if opts.Server != nil {
		s.server, err = opts.Server.Apply()
		if err != nil {
			return nil, errors.WithStack(err)
		}
	} else {
		s.server, _ = new(options.HTTP_Server).Apply()
	}
	s.server.Handler = s
	s.server.ErrorLog = logger.Wrap(logger.ERROR, s.tag, l)
	// client transport
	if opts.Transport != nil {
		s.transport, err = opts.Transport.Apply()
		if err != nil {
			return nil, errors.WithStack(err)
		}
	} else {
		s.transport, _ = new(options.HTTP_Transport).Apply()
	}
	if opts.Limit > 0 {
		s.limit = opts.Limit
	}
	// basic authentication
	if opts.Username != "" {
		s.basic_auth = []byte(base64.StdEncoding.EncodeToString(
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

func (this *Server) Listen_And_Serve(address string, start_timeout time.Duration) error {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	return this.Serve(tcpKeepAliveListener{l.(*net.TCPListener)}, start_timeout)
}

func (this *Server) Serve(l net.Listener, start_timeout time.Duration) error {
	defer this.m.Unlock()
	this.m.Lock()
	this.addr = l.Addr().String()
	limit_l := netutil.LimitListener(l, this.limit)
	return this.start(func() error { return this.server.Serve(limit_l) }, start_timeout)
}

func (this *Server) start(f func() error, start_timeout time.Duration) error {
	if start_timeout < 1 {
		start_timeout = options.DEFAULT_START_TIMEOUT
	}
	err_chan := make(chan error, 1)
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
			err_chan <- err
			close(err_chan)
		}()
		err = f()
	}()
	select {
	case err := <-err_chan:
		this.log(logger.INFO, "start server failed:", err)
		return err
	case <-time.After(start_timeout):
		this.log(logger.INFO, "start server success: ", this.addr)
		return nil
	}
}

func (this *Server) Stop() error {
	err := this.server.Close()
	this.transport.CloseIdleConnections()
	this.log(logger.INFO, "server stopped")
	return err
}

func (this *Server) Info() string {
	defer this.m.Unlock()
	this.m.Lock()
	auth, _ := base64.StdEncoding.DecodeString(string(this.basic_auth))
	return fmt.Sprintf("Listen: %s Auth: %s", this.addr, auth)
}

func (this *Server) Addr() string {
	defer this.m.Unlock()
	this.m.Lock()
	return this.addr
}

func (this *Server) log(l logger.Level, log ...interface{}) {
	this.logger.Println(l, this.tag, log...)
}

type serve_log struct {
	Log interface{}
	R   *http.Request
}

func (this *serve_log) String() string {
	return fmt.Sprint(this.Log, "\n", logger.HTTP_Request(this.R))
}

func (this *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			this.log(logger.ERROR, &serve_log{Log: fmt.Sprint("panic: ", rec), R: r})
		}
	}()
	// auth
	if this.basic_auth != nil {
		auth_failed := func() {
			w.Header().Set("Proxy-Authenticate", "Basic")
			w.WriteHeader(http.StatusProxyAuthRequired)
		}
		auth := strings.Split(r.Header.Get("Proxy-Authorization"), " ")
		if len(auth) != 2 {
			// not log
			auth_failed()
			return
		}
		auth_method := auth[0]
		auth_base64 := auth[1]
		switch auth_method {
		case "Basic":
			if subtle.ConstantTimeCompare(this.basic_auth, []byte(auth_base64)) != 1 {
				auth_failed()
				this.log(logger.EXPLOIT, &serve_log{Log: "invalid basic authenticate", R: r})
				return
			}
		default: // not support method
			auth_failed()
			this.log(logger.EXPLOIT, &serve_log{Log: "unsupport auth method: " + auth_method, R: r})
			return
		}
	}
	// handle https
	if r.Method == http.MethodConnect {
		// dial
		conn, err := this.transport.DialContext(context.Background(), "tcp", r.URL.Host)
		if err != nil {
			this.log(logger.ERROR, &serve_log{Log: err, R: r})
			return
		}
		// get client conn
		wc, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			this.log(logger.ERROR, &serve_log{Log: err, R: r})
			return
		}
		_, err = wc.Write(connection_established)
		if err != nil {
			this.log(logger.ERROR, &serve_log{Log: err, R: r})
			return
		}
		go func() { _, _ = io.Copy(conn, wc) }()
		_, _ = io.Copy(wc, conn)
		_ = conn.Close()
		_ = wc.Close()
	} else { // handle http
		resp, err := this.transport.RoundTrip(r)
		if err != nil {
			this.log(logger.ERROR, &serve_log{Log: err, R: r})
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
