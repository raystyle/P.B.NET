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

	"project/internal/logger"
	"project/internal/options"
	"project/internal/xpanic"
)

type Options struct {
	HTTPS    bool          `toml:"https"`
	Username string        `toml:"username"`
	Password string        `toml:"password"`
	Timeout  time.Duration `toml:"timeout"`

	// only client
	Header    http.Header       `toml:"header"`
	TLSConfig options.TLSConfig `toml:"tls_config"`

	// only server
	MaxConns  int                   `toml:"max_conns"`
	Server    options.HTTPServer    `toml:"server"`
	Transport options.HTTPTransport `toml:"transport"`

	DialContext func(ctx context.Context, network, address string) (net.Conn, error) `toml:"-"`
	ExitFunc    func()                                                               `toml:"-"`
}

// Server implement internal/proxy.server
type Server struct {
	tag         string
	logger      logger.Logger
	https       bool
	timeout     time.Duration
	maxConns    int
	dialContext func(ctx context.Context, network, address string) (net.Conn, error)
	exitFunc    func()
	execOnce    sync.Once

	server    *http.Server
	transport *http.Transport
	username  []byte
	password  []byte
	address   string
	rwm       sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	closeOnce sync.Once
	wg        sync.WaitGroup
}

func NewServer(tag string, lg logger.Logger, opts *Options) (*Server, error) {
	if tag == "" {
		return nil, errors.New("empty tag")
	}
	if opts == nil {
		opts = new(Options)
	}
	s := Server{
		logger:      lg,
		https:       opts.HTTPS,
		maxConns:    opts.MaxConns,
		dialContext: opts.DialContext,
		exitFunc:    opts.ExitFunc,
	}
	var err error
	s.server, err = opts.Server.Apply()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	timeout := opts.Timeout
	if timeout < 1 {
		timeout = options.DefaultDialTimeout
	}
	s.server.ReadTimeout = timeout
	s.server.WriteTimeout = timeout
	s.timeout = timeout

	s.transport, err = opts.Transport.Apply()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if opts.Username != "" {
		s.username = []byte(opts.Username)
	}
	if opts.Password != "" {
		s.password = []byte(opts.Password)
	}

	if s.https {
		s.tag = "https proxy-" + tag
	} else {
		s.tag = "http proxy-" + tag
	}

	if s.maxConns < 1 {
		s.maxConns = options.DefaultMaxConns
	}

	if s.dialContext == nil {
		s.dialContext = new(net.Dialer).DialContext
	}

	if opts.DialContext != nil {
		s.transport.DialContext = opts.DialContext
	}

	s.server.Handler = &s
	s.server.ErrorLog = logger.Wrap(logger.Error, s.tag, lg)
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return &s, nil
}

func (s *Server) ListenAndServe(network, address string) error {
	// check network
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return errors.Errorf("unsupport network: %s", network)
	}
	// listen
	l, err := net.Listen(network, address)
	if err != nil {
		return errors.WithStack(err)
	}
	s.Serve(l)
	return nil
}

func (s *Server) Serve(l net.Listener) {
	s.rwm.Lock()
	defer s.rwm.Unlock()
	s.address = l.Addr().String()
	ll := netutil.LimitListener(l, s.maxConns)
	s.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.log(logger.Fatal, xpanic.Print(r, "Server.Serve()"))
			}
			s.cancel()
			s.transport.CloseIdleConnections()
			s.rwm.Lock()
			defer s.rwm.Unlock()
			s.server = nil // must use it
			s.doExitFunc()
			s.logf(logger.Info, "server stopped (%s)", s.address)
			s.wg.Done()
		}()
		s.logf(logger.Info, "start server (%s)", s.address)
		if s.https {
			_ = s.server.ServeTLS(ll, "", "")
		} else {
			_ = s.server.Serve(ll)
		}
	}()
}

func (s *Server) Close() error {
	var err error
	s.closeOnce.Do(func() {
		err = s.server.Close()
		s.wg.Wait()
		s.doExitFunc()
	})
	return err
}

func (s *Server) Address() string {
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	return s.address
}

// Info is used to get http proxy server info
// "http proxy listen: 127.0.0.1:8080"
// "http proxy listen: 127.0.0.1:8080 admin:123456"
func (s *Server) Info() string {
	buf := new(bytes.Buffer)
	if s.https {
		buf.WriteString("https")
	} else {
		buf.WriteString("http")
	}
	_, _ = fmt.Fprintf(buf, " proxy listen: %s", s.Address())
	if s.username != nil || s.password != nil {
		_, _ = fmt.Fprintf(buf, " %s:%s", s.username, s.password)
	}
	return buf.String()
}

func (s *Server) doExitFunc() {
	s.execOnce.Do(func() {
		if s.exitFunc != nil {
			s.exitFunc()
		}
	})
}

func (s *Server) log(lv logger.Level, log ...interface{}) {
	l := len(log)
	logs := make([]interface{}, l)
	for i := 0; i < l; i++ {
		if r, ok := log[i].(*http.Request); ok {
			logs[i] = logger.HTTPRequest(r)
		} else {
			logs[i] = log[i]
		}
	}
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprint(buf, logs...)
	s.logger.Println(lv, s.tag, buf)
}

func (s *Server) logf(lv logger.Level, format string, log ...interface{}) {
	s.logger.Printf(lv, s.tag, format, log...)
}

var (
	connectionEstablished    = []byte("HTTP/1.0 200 Connection established\r\n\r\n")
	connectionEstablishedLen = len(connectionEstablished)
)

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const title = "Server.ServeHTTP()"
	s.wg.Add(1)
	defer func() {
		if rec := recover(); rec != nil {
			s.log(logger.Fatal, xpanic.Print(rec, title), "\n", r)
		}
		s.wg.Done()
	}()
	// authenticate
	if s.username != nil || s.password != nil {
		authFailed := func() {
			w.Header().Set("Proxy-Authenticate", "Basic")
			w.WriteHeader(http.StatusProxyAuthRequired)
		}
		auth := strings.Split(r.Header.Get("Proxy-Authorization"), " ")
		if len(auth) != 2 {
			authFailed()
			return
		}
		authMethod := auth[0]
		authBase64 := auth[1]
		switch authMethod {
		case "Basic":
			auth, err := base64.StdEncoding.DecodeString(authBase64)
			if err != nil {
				authFailed()
				s.log(logger.Exploit, "invalid basic base64 data\n", r)
				return
			}
			userPass := strings.Split(string(auth), ":")
			if len(userPass) < 2 {
				userPass = append(userPass, "")
			}
			if subtle.ConstantTimeCompare(s.username, []byte(userPass[0])) != 1 ||
				subtle.ConstantTimeCompare(s.password, []byte(userPass[1])) != 1 {
				authFailed()
				s.log(logger.Exploit, "invalid basic authenticate\n", r)
				return
			}
		default: // not support method
			authFailed()
			s.log(logger.Exploit, "unsupport auth method: "+authMethod+"\n", r)
			return
		}
	}
	// remove Proxy-Authorization
	r.Header.Del("Proxy-Authorization")
	s.log(logger.Debug, "handle request\n", r)
	if r.Method == http.MethodConnect { // handle https
		var err error
		defer func() {
			if err != nil {
				s.log(logger.Error, err, "\n", r)
			}
		}()
		// hijack client conn
		wc, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer func() { _ = wc.Close() }()
		ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
		defer cancel()
		// dial target
		conn, err := s.dialContext(ctx, "tcp", r.URL.Host)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, err = wc.Write(connectionEstablished)
		if err != nil {
			return
		}
		// http.Server.Close() not close hijacked conn
		closeChan := make(chan struct{})
		s.wg.Add(1)
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					s.log(logger.Fatal, xpanic.Print(rec, title), "\n", r)
				}
				s.wg.Done()
			}()
			select {
			case <-closeChan:
			case <-s.ctx.Done():
				_ = wc.Close()
				_ = conn.Close()
			}
		}()
		// start copy
		s.wg.Add(1)
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					s.log(logger.Fatal, xpanic.Print(rec, title), "\n", r)
				}
				s.wg.Done()
			}()
			_, _ = io.Copy(conn, wc)
		}()
		_, _ = io.Copy(wc, conn)
		close(closeChan)
	} else { // handle http
		ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
		defer cancel()
		resp, err := s.transport.RoundTrip(r.Clone(ctx))
		if err != nil {
			s.log(logger.Error, err, "\n", r)
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
