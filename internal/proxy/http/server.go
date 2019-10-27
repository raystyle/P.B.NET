package http

import (
	"bytes"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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
	ExitFunc  func()                `toml:"-"`
}

// Server implement internal/proxy.Server
type Server struct {
	tag      string
	logger   logger.Logger
	https    bool
	maxConns int
	exitFunc func()
	execOnce sync.Once

	server    *http.Server
	transport *http.Transport
	address   string
	basicAuth []byte
	rwm       sync.RWMutex

	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func NewServer(tag string, lg logger.Logger, opts *Options) (*Server, error) {
	if tag == "" {
		return nil, errors.New("empty tag")
	}
	if opts == nil {
		opts = new(Options)
	}
	s := Server{
		logger:   lg,
		https:    opts.HTTPS,
		maxConns: opts.MaxConns,
		exitFunc: opts.ExitFunc,
	}
	var err error

	s.server, err = opts.Server.Apply()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	timeout := opts.Timeout
	if timeout < 1 {
		timeout = options.DefaultDeadline
	}
	s.server.ReadTimeout = timeout
	s.server.WriteTimeout = timeout

	s.transport, err = opts.Transport.Apply()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if s.https {
		s.tag = "https proxy-" + tag
	} else {
		s.tag = "http proxy-" + tag
	}

	if s.maxConns < 1 {
		s.maxConns = options.DefaultMaxConns
	}

	// basic authentication
	var auth string
	if opts.Username != "" && opts.Password != "" {
		auth = url.UserPassword(opts.Username, opts.Password).String()
	} else if opts.Username != "" {
		auth = url.User(opts.Username).String()
	}
	if auth != "" {
		s.basicAuth = []byte(base64.StdEncoding.EncodeToString([]byte(auth)))
	}

	s.server.Handler = &s
	s.server.ErrorLog = logger.Wrap(logger.Error, s.tag, lg)
	s.stopSignal = make(chan struct{})
	return &s, nil
}

func (s *Server) ListenAndServe(network, address string) error {
	l, err := net.Listen(network, address)
	if err != nil {
		return err
	}
	s.Serve(l)
	return nil
}

func (s *Server) Serve(l net.Listener) {
	s.rwm.Lock()
	s.address = l.Addr().String()
	s.rwm.Unlock()
	ll := netutil.LimitListener(l, s.maxConns)
	s.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.log(logger.Fatal, xpanic.Print(r, "Server.Serve()"))
			}
			close(s.stopSignal)
			s.transport.CloseIdleConnections()
			s.rwm.Lock()
			s.server = nil
			s.rwm.Unlock()
			s.execOnce.Do(func() {
				if s.exitFunc != nil {
					s.exitFunc()
				}
			})
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

func (s *Server) Close() (err error) {
	s.rwm.RLock()
	server := s.server
	s.rwm.RUnlock()
	if server != nil {
		err = server.Close()
		s.wg.Wait()
		s.execOnce.Do(func() {
			if s.exitFunc != nil {
				s.exitFunc()
			}
		})
	}
	return err
}

func (s *Server) Address() string {
	s.rwm.RLock()
	addr := s.address
	s.rwm.RUnlock()
	return addr
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
	if s.basicAuth != nil {
		auth, _ := base64.StdEncoding.DecodeString(string(s.basicAuth))
		buf.WriteString(" ")
		buf.Write(auth)
	}
	return buf.String()
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
	if s.basicAuth != nil {
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
			if subtle.ConstantTimeCompare(s.basicAuth, []byte(authBase64)) != 1 {
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
		// dial target
		conn, err := net.Dial("tcp", r.URL.Host)
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
			case <-s.stopSignal:
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
		// remove Proxy-Authorization
		r.Header.Del("Proxy-Authorization")
		resp, err := s.transport.RoundTrip(r)
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
