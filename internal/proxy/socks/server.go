package socks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/logger"
	"project/internal/options"
	"project/internal/xpanic"
)

type Options struct {
	Socks4   bool          `toml:"socks4"`
	Username string        `toml:"username"` // useless for socks4
	Password string        `toml:"password"` // useless for socks4
	Timeout  time.Duration `toml:"timeout"`  // handshake timeout
	UserID   string        `toml:"user_id"`  // useless for socks5

	// only client
	DisableSocks4A bool `toml:"disable_socks4a"`

	// only server
	MaxConns int `toml:"max_conns"`

	DialContext func(ctx context.Context, network, address string) (net.Conn, error) `toml:"-"`
	ExitFunc    func()                                                               `toml:"-"`
}

// Server implement internal/proxy.server
type Server struct {
	tag      string
	logger   logger.Logger
	socks4   bool
	maxConns int
	timeout  time.Duration
	userID   []byte

	dialContext func(ctx context.Context, network, address string) (net.Conn, error)
	exitFunc    func()

	listener net.Listener
	username []byte
	password []byte
	address  string
	rwm      sync.RWMutex
	conns    map[string]*conn
	connsRWM sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	inShutdown int32
	closeOnce  sync.Once
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
		logger:      lg,
		socks4:      opts.Socks4,
		maxConns:    opts.MaxConns,
		timeout:     opts.Timeout,
		userID:      []byte(opts.UserID),
		conns:       make(map[string]*conn),
		dialContext: opts.DialContext,
		exitFunc:    opts.ExitFunc,
	}

	if s.socks4 {
		s.tag = "socks4a-" + tag
	} else {
		s.tag = "socks5-" + tag
	}

	if s.timeout < 1 {
		s.timeout = options.DefaultHandshakeTimeout
	}

	if s.maxConns < 1 {
		s.maxConns = options.DefaultMaxConns
	}

	if s.dialContext == nil {
		s.dialContext = new(net.Dialer).DialContext
	}

	if opts.Username != "" {
		s.username = []byte(opts.Username)
		s.password = []byte(opts.Password)
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	return &s, nil
}

func (s *Server) ListenAndServe(network, address string) error {
	// check network
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return errors.Errorf("unsupported network: %s", network)
	}
	// listen
	l, err := net.Listen(network, address)
	if err != nil {
		return errors.WithStack(err)
	}
	s.Serve(l)
	return nil
}

func (s *Server) close(err error) {
	if r := recover(); r != nil {
		s.log(logger.Fatal, xpanic.Print(r, "Server.Serve()"))
	}
	if err != nil {
		s.log(logger.Error, err)
	}

	atomic.StoreInt32(&s.inShutdown, 1)
	s.cancel()

	// close all connections
	s.connsRWM.Lock()
	defer s.connsRWM.Unlock()
	for _, conn := range s.conns {
		_ = conn.local.Close()
	}

	// close listener and execute exit function
	s.closeOnce.Do(func() {
		_ = s.listener.Close()
		if s.exitFunc != nil {
			s.exitFunc()
		}
	})

	s.logf(logger.Info, "server stopped (%s)", s.address)
	s.wg.Done()
}

func (s *Server) Serve(l net.Listener) {
	s.rwm.Lock()
	defer s.rwm.Unlock()
	s.address = l.Addr().String()
	s.listener = netutil.LimitListener(l, s.maxConns)

	s.wg.Add(1)
	go func() {
		var err error
		defer s.close(err)
		s.logf(logger.Info, "start server (%s)", s.address)
		var delay time.Duration // how long to sleep on accept failure
		maxDelay := time.Second
		for {
			conn, e := s.listener.Accept()
			if e != nil {
				// check error
				if ne, ok := e.(net.Error); ok && ne.Temporary() {
					if delay == 0 {
						delay = 5 * time.Millisecond
					} else {
						delay *= 2
					}
					if delay > maxDelay {
						delay = maxDelay
					}
					s.logf(logger.Warning, "accept error: %s; retrying in %v", e, delay)
					time.Sleep(delay)
					continue
				}
				if !strings.Contains(e.Error(), "use of closed network connection") {
					err = e
				}
				return
			}
			delay = 0
			c := s.newConn(conn)
			if c != nil {
				_ = conn.SetDeadline(time.Now().Add(s.timeout))
				s.wg.Add(1)
				go c.serve()
			}
		}
	}()
}

func (s *Server) Close() error {
	var err error
	atomic.StoreInt32(&s.inShutdown, 1)
	s.closeOnce.Do(func() {
		s.rwm.RLock()
		defer s.rwm.RUnlock()
		if s.listener != nil {
			err = s.listener.Close()
		}
		if s.exitFunc != nil {
			s.exitFunc()
		}
	})
	s.wg.Wait()
	return err
}

func (s *Server) Address() string {
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	return s.address
}

// Info is used to get http proxy server info
// "socks5 listen: 127.0.0.1:8080"
// "socks5 listen: 127.0.0.1:8080 admin:123456"
func (s *Server) Info() string {
	buf := new(bytes.Buffer)
	if s.socks4 {
		buf.WriteString("socks4a")
	} else {
		buf.WriteString("socks5")
	}
	_, _ = fmt.Fprintf(buf, " listen: %s", s.Address())
	if s.username != nil || s.password != nil {
		_, _ = fmt.Fprintf(buf, " %s:%s", s.username, s.password)
	}
	return buf.String()
}

func (s *Server) log(lv logger.Level, log ...interface{}) {
	l := len(log)
	logs := make([]interface{}, l)
	for i := 0; i < l; i++ {
		if c, ok := log[i].(net.Conn); ok {
			logs[i] = logger.Conn(c)
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

func (s *Server) newConn(c net.Conn) *conn {
	if atomic.LoadInt32(&s.inShutdown) == 0 {
		conn := &conn{
			server: s,
			local:  c,
		}
		s.connsRWM.Lock()
		defer s.connsRWM.Unlock()
		s.conns[conn.key()] = conn
		return conn
	}
	_ = c.Close()
	return nil
}

type conn struct {
	server *Server
	local  net.Conn // listener accepted conn
	remote net.Conn // dial
}

func (c *conn) key() string {
	return fmt.Sprintf("%s%s%s%s",
		c.local.LocalAddr().Network(), c.local.LocalAddr(),
		c.local.RemoteAddr().Network(), c.local.RemoteAddr(),
	)
}

func (c *conn) log(lv logger.Level, log ...interface{}) {
	c.server.log(lv, append(log, "\n", c.local)...)
}

func (c *conn) serve() {
	const title = "conn.serve()"
	defer func() {
		if r := recover(); r != nil {
			c.log(logger.Fatal, xpanic.Print(r, title))
		}
		_ = c.local.Close()
		// delete conn
		c.server.connsRWM.Lock()
		defer c.server.connsRWM.Unlock()
		delete(c.server.conns, c.key())
		c.server.wg.Done()
	}()
	if c.server.socks4 {
		c.serveSocks4()
	} else {
		c.serveSocks5()
	}
	// start copy
	if c.remote != nil {
		defer func() { _ = c.remote.Close() }()
		_ = c.remote.SetDeadline(time.Time{})
		_ = c.local.SetDeadline(time.Time{})
		c.server.wg.Add(1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					c.log(logger.Fatal, xpanic.Print(r, title))
				}
				c.server.wg.Done()
			}()
			_, _ = io.Copy(c.local, c.remote)
		}()
		_, _ = io.Copy(c.remote, c.local)
	}
}
