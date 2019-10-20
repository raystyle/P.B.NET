package http

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/logger"
	"project/internal/options"
	"project/internal/xpanic"
)

type Options struct {
	Username  string
	Password  string
	MaxConns  int
	Server    options.HTTPServer
	Transport options.HTTPTransport
	ExitFunc  func()
}

type Server struct {
	tag      string
	logger   logger.Logger
	maxConns int
	exitFunc func()

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
	s := &Server{
		tag:      "http proxy-" + tag,
		logger:   lg,
		maxConns: opts.MaxConns,
	}
	var err error
	// server
	s.server, err = opts.Server.Apply()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// transport
	s.transport, err = opts.Transport.Apply()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if s.maxConns < 1 {
		s.maxConns = options.DefaultMaxConns
	}
	// basic authentication
	if opts.Username != "" {
		auth := []byte(opts.Username + ":" + opts.Password)
		s.basicAuth = []byte(base64.StdEncoding.EncodeToString(auth))
	}
	s.server.Handler = s
	s.server.ErrorLog = logger.Wrap(logger.Error, s.tag, lg)
	s.stopSignal = make(chan struct{})
	return s, nil
}

func (s *Server) ListenAndServe(address string) error {
	l, err := net.Listen("tcp", address)
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
				s.log(logger.Fatal, xpanic.Sprint(r, "Server.Serve()"))
			}
			close(s.stopSignal)
			s.transport.CloseIdleConnections()
			s.rwm.Lock()
			s.server = nil
			s.rwm.Unlock()
			// exit func
			if s.exitFunc != nil {
				s.exitFunc()
			}
			s.log(logger.Info, "server stopped")
			s.wg.Done()
		}()
		s.logf(logger.Info, "start server (%s)", s.address)
		_ = s.server.Serve(ll)
	}()
}

func (s *Server) Close() (err error) {
	s.rwm.RLock()
	p := s.server
	s.rwm.RUnlock()
	if p != nil {
		err = p.Close()
		s.wg.Wait()
	}
	return err
}

func (s *Server) Address() string {
	s.rwm.RLock()
	addr := s.address
	s.rwm.RUnlock()
	return addr
}

func (s *Server) Info() string {
	s.rwm.RLock()
	auth, _ := base64.StdEncoding.DecodeString(string(s.basicAuth))
	addr := s.address
	s.rwm.RUnlock()
	return fmt.Sprintf("listen: %s auth: %s", addr, auth)
}

func (s *Server) log(lv logger.Level, log ...interface{}) {
	s.logger.Println(lv, s.tag, log...)
}

func (s *Server) logf(lv logger.Level, format string, log ...interface{}) {
	s.logger.Printf(lv, s.tag, format, log...)
}

type log struct {
	Log interface{}
	Req *http.Request
}

func (l *log) String() string {
	return fmt.Sprint(l.Log, "\n", logger.HTTPRequest(l.Req))
}

var (
	connectionEstablished = []byte("HTTP/1.1 200 Connection established\r\n\r\n")
)

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.wg.Add(1)
	defer func() {
		if rec := recover(); rec != nil {
			l := &log{Log: xpanic.Sprint(rec, "Server.ServeHTTP()"), Req: r}
			s.log(logger.Fatal, l)
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
				s.log(logger.Exploit, &log{Log: "invalid basic authenticate", Req: r})
				return
			}
		default: // not support method
			authFailed()
			s.log(logger.Exploit, &log{Log: "unsupport auth method: " + authMethod, Req: r})
			return
		}
	}
	s.log(logger.Debug, &log{Log: "handle request", Req: r})
	if r.Method == http.MethodConnect { // handle https
		var err error
		defer func() {
			if err != nil {
				s.log(logger.Error, &log{Log: err, Req: r})
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
					l := &log{Log: xpanic.Sprint(rec, "Server.ServeHTTP()"), Req: r}
					s.log(logger.Fatal, l)
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
					l := &log{Log: xpanic.Sprint(rec, "Server.ServeHTTP()"), Req: r}
					s.log(logger.Fatal, l)
				}
				s.wg.Done()
			}()
			_, _ = io.Copy(conn, wc)
		}()
		_, _ = io.Copy(wc, conn)
		close(closeChan)
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
