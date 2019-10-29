package socks

import (
	"bytes"
	"fmt"
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
	MaxConns int    `toml:"max_conns"`
	ExitFunc func() `toml:"-"`
}

// Server implement internal/proxy.server
type Server struct {
	tag      string
	logger   logger.Logger
	socks4   bool
	maxConns int
	timeout  time.Duration
	userID   []byte
	exitFunc func()

	listener net.Listener
	username []byte
	password []byte
	address  string
	rwm      sync.RWMutex
	conns    map[string]*conn
	connsRWM sync.RWMutex

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
		logger:   lg,
		socks4:   opts.Socks4,
		maxConns: opts.MaxConns,
		timeout:  opts.Timeout,
		userID:   []byte(opts.UserID),
		conns:    make(map[string]*conn),
		exitFunc: opts.ExitFunc,
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

	if opts.Username != "" {
		s.username = []byte(opts.Username)
		s.password = []byte(opts.Password)
	}
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
		return err
	}
	s.Serve(l)
	return nil
}

func (s *Server) Serve(l net.Listener) {
	s.rwm.Lock()
	s.address = l.Addr().String()
	s.listener = netutil.LimitListener(l, s.maxConns)
	s.rwm.Unlock()
	s.wg.Add(1)
	go func() {
		var err error
		defer func() {
			atomic.StoreInt32(&s.inShutdown, 1)
			if r := recover(); r != nil {
				s.log(logger.Fatal, xpanic.Print(r, "Server.Serve()"))
			}
			if err != nil {
				s.log(logger.Error, err)
			}

			// close all connections
			s.connsRWM.Lock()
			for _, conn := range s.conns {
				_ = conn.conn.Close()
			}
			s.connsRWM.Unlock()

			// close listener and execute exit function
			s.closeOnce.Do(func() {
				err = s.listener.Close()
				if s.exitFunc != nil {
					s.exitFunc()
				}
			})

			s.logf(logger.Info, "server stopped (%s)", s.address)
			s.wg.Done()
		}()
		s.logf(logger.Info, "start server (%s)", s.address)
		var delay time.Duration // how long to sleep on accept failure
		maxDelay := 1 * time.Second
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

func (s *Server) Close() (err error) {
	atomic.StoreInt32(&s.inShutdown, 1)
	s.closeOnce.Do(func() {
		s.rwm.RLock()
		l := s.listener
		s.rwm.RUnlock()
		if l != nil {
			err = l.Close()
		}
		if s.exitFunc != nil {
			s.exitFunc()
		}
	})
	s.wg.Wait()
	return
}

func (s *Server) Address() string {
	s.rwm.RLock()
	addr := s.address
	s.rwm.RUnlock()
	return addr
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
	if s.username != nil {
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
			conn:   c,
		}
		s.connsRWM.Lock()
		s.conns[conn.key()] = conn
		s.connsRWM.Unlock()
		return conn
	}
	_ = c.Close()
	return nil
}

type conn struct {
	server *Server
	conn   net.Conn
}

func (c *conn) key() string {
	return fmt.Sprintf("%s%s%s%s",
		c.conn.LocalAddr().Network(), c.conn.LocalAddr(),
		c.conn.RemoteAddr().Network(), c.conn.RemoteAddr(),
	)
}

func (c *conn) log(lv logger.Level, log ...interface{}) {
	c.server.log(lv, append(log, "\n", c.conn)...)
}

func (c *conn) serve() {
	defer func() {
		if r := recover(); r != nil {
			c.log(logger.Fatal, xpanic.Print(r, "conn.serve()"))
		}
		_ = c.conn.Close()
		// delete conn
		c.server.connsRWM.Lock()
		delete(c.server.conns, c.key())
		c.server.connsRWM.Unlock()
		c.server.wg.Done()
	}()
	if c.server.socks4 {
		c.serveSocks4()
	} else {
		c.serveSocks5()
	}
}
