package socks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/logger"
	"project/internal/nettool"
	"project/internal/xpanic"
)

// ErrServerClosed is returned by the Server's Serve, ListenAndServe,
// methods after a call Close.
var ErrServerClosed = errors.New("socks server closed")

// Server implemented internal/proxy.server.
type Server struct {
	tag        string
	logger     logger.Logger
	socks4     bool
	disableExt bool // socks4 can't resolve domain name

	// options
	username []byte
	password []byte
	userID   []byte
	timeout  time.Duration
	maxConns int

	// secondary proxy
	dialContext func(ctx context.Context, network, address string) (net.Conn, error)

	listeners  map[*net.Listener]struct{}
	conns      map[*conn]struct{}
	inShutdown int32
	rwm        sync.RWMutex

	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewSocks5Server is used to create a socks5 server.
func NewSocks5Server(tag string, lg logger.Logger, opts *Options) (*Server, error) {
	return newServer(tag, lg, opts, false, false)
}

// NewSocks4aServer is used to create a socks4a server.
func NewSocks4aServer(tag string, lg logger.Logger, opts *Options) (*Server, error) {
	return newServer(tag, lg, opts, true, false)
}

// NewSocks4Server is used to create a socks4 server.
func NewSocks4Server(tag string, lg logger.Logger, opts *Options) (*Server, error) {
	return newServer(tag, lg, opts, true, true)
}

func newServer(tag string, lg logger.Logger, opts *Options, socks4, disableExt bool) (*Server, error) {
	if tag == "" {
		return nil, errors.New("empty tag")
	}
	if opts == nil {
		opts = new(Options)
	}
	srv := Server{
		logger:      lg,
		socks4:      socks4,
		disableExt:  disableExt,
		timeout:     opts.Timeout,
		maxConns:    opts.MaxConns,
		dialContext: opts.DialContext,
		listeners:   make(map[*net.Listener]struct{}),
		conns:       make(map[*conn]struct{}),
	}
	// tag
	if srv.socks4 {
		if srv.disableExt {
			srv.tag = "socks4-" + tag
		} else {
			srv.tag = "socks4a-" + tag
		}
	} else {
		srv.tag = "socks5-" + tag
	}
	// auth
	if opts.Username != "" || opts.Password != "" {
		srv.username = []byte(opts.Username)
		srv.password = []byte(opts.Password)
	}
	if opts.UserID != "" {
		srv.userID = []byte(opts.UserID)
	}
	if srv.timeout < 1 {
		srv.timeout = defaultConnectTimeout
	}
	if srv.maxConns < 1 {
		srv.maxConns = defaultMaxConnections
	}
	if srv.dialContext == nil {
		srv.dialContext = new(net.Dialer).DialContext
	}
	srv.ctx, srv.cancel = context.WithCancel(context.Background())
	return &srv, nil
}

func (s *Server) logf(lv logger.Level, format string, log ...interface{}) {
	s.logger.Printf(lv, s.tag, format, log...)
}

func (s *Server) log(lv logger.Level, log ...interface{}) {
	s.logger.Println(lv, s.tag, log...)
}

func (s *Server) shuttingDown() bool {
	return atomic.LoadInt32(&s.inShutdown) != 0
}

func (s *Server) trackListener(listener *net.Listener, add bool) bool {
	s.rwm.Lock()
	defer s.rwm.Unlock()
	if add {
		if s.shuttingDown() {
			return false
		}
		s.listeners[listener] = struct{}{}
	} else {
		delete(s.listeners, listener)
	}
	return true
}

// ListenAndServe is used to listen a listener and serve.
func (s *Server) ListenAndServe(network, address string) error {
	if s.shuttingDown() {
		return ErrServerClosed
	}
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
	s.wg.Add(1)
	defer s.wg.Done()

	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "Server.Serve")
			s.log(logger.Fatal, err)
		}
	}()

	address := listener.Addr()
	network := address.Network()

	listener = netutil.LimitListener(listener, s.maxConns)
	defer func() {
		err := listener.Close()
		if err != nil && !nettool.IsNetClosingError(err) {
			const format = "failed to close listener (%s %s): %s"
			s.logf(logger.Error, format, network, address, err)
		}
	}()

	if !s.trackListener(&listener, true) {
		return ErrServerClosed
	}
	defer s.trackListener(&listener, false)

	s.logf(logger.Info, "start listener (%s %s)", network, address)
	defer s.logf(logger.Info, "listener closed (%s %s)", network, address)

	// start accept loop
	var delay time.Duration // how long to sleep on accept failure
	maxDelay := time.Second
	for {
		conn, err := listener.Accept()
		if err != nil {
			// check error
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if delay == 0 {
					delay = 5 * time.Millisecond
				} else {
					delay *= 2
				}
				if delay > maxDelay {
					delay = maxDelay
				}
				s.logf(logger.Warning, "accept error: %s; retrying in %v", err, delay)
				time.Sleep(delay)
				continue
			}
			if nettool.IsNetClosingError(err) {
				return nil
			}
			s.log(logger.Error, err)
			return err
		}
		delay = 0
		s.newConn(conn).Serve()
	}
}

func (s *Server) newConn(c net.Conn) *conn {
	return &conn{
		server: s,
		local:  c,
	}
}

func (s *Server) trackConn(conn *conn, add bool) bool {
	s.rwm.Lock()
	defer s.rwm.Unlock()
	if add {
		if s.shuttingDown() {
			return false
		}
		s.conns[conn] = struct{}{}
	} else {
		delete(s.conns, conn)
	}
	return true
}

// Addresses is used to get listener addresses.
func (s *Server) Addresses() []net.Addr {
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	addresses := make([]net.Addr, 0, len(s.listeners))
	for listener := range s.listeners {
		addresses = append(addresses, (*listener).Addr())
	}
	return addresses
}

// Info is used to get socks server information.
// "address: tcp 127.0.0.1:1999, tcp4 127.0.0.1:2001"
// "address: tcp 127.0.0.1:1999 user id: test"
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
	if s.socks4 {
		if s.userID != nil {
			format := "user id: %s"
			if buf.Len() > 0 {
				format = " " + format
			}
			_, _ = fmt.Fprintf(buf, format, s.userID)
		}
	} else {
		if s.username != nil || s.password != nil {
			format := "auth: %s:%s"
			if buf.Len() > 0 {
				format = " " + format
			}
			_, _ = fmt.Fprintf(buf, format, s.username, s.password)
		}
	}
	return buf.String()
}

// Close is used to close socks server.
func (s *Server) Close() error {
	var err error
	s.closeOnce.Do(func() {
		atomic.StoreInt32(&s.inShutdown, 1)
		s.cancel()
		s.rwm.Lock()
		defer s.rwm.Unlock()
		// close all listeners
		for listener := range s.listeners {
			e := (*listener).Close()
			if e != nil && err == nil {
				err = e
			}
		}
		// close all connections
		for conn := range s.conns {
			e := conn.Close()
			if e != nil && err == nil {
				err = e
			}
		}
	})
	s.wg.Wait()
	return err
}

type conn struct {
	server *Server
	local  net.Conn // listener accepted conn
	remote net.Conn // dial
}

func (c *conn) log(lv logger.Level, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintln(buf, log...)
	_, _ = logger.Conn(c.local).WriteTo(buf)
	c.server.log(lv, buf)
}

func (c *conn) Serve() {
	c.server.wg.Add(1)
	go c.serve()
}

func (c *conn) serve() {
	defer c.server.wg.Done()

	const title = "conn.serve()"
	defer func() {
		if r := recover(); r != nil {
			c.log(logger.Fatal, xpanic.Print(r, title))
		}
	}()

	defer func() {
		err := c.local.Close()
		if err != nil && !nettool.IsNetClosingError(err) {
			c.log(logger.Error, "failed to close local connection:", err)
		}
	}()

	if !c.server.trackConn(c, true) {
		return
	}
	defer c.server.trackConn(c, false)

	// handle
	_ = c.local.SetDeadline(time.Now().Add(c.server.timeout))
	if c.server.socks4 {
		c.serveSocks4()
	} else {
		c.serveSocks5()
	}
	if c.remote == nil {
		return
	}

	defer func() {
		err := c.remote.Close()
		if err != nil && !nettool.IsNetClosingError(err) {
			c.log(logger.Error, "failed to close remote connection:", err)
		}
	}()

	// reset deadline
	_ = c.remote.SetDeadline(time.Time{})
	_ = c.local.SetDeadline(time.Time{})

	// start copy
	c.server.wg.Add(1)
	go func() {
		defer c.server.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				c.log(logger.Fatal, xpanic.Print(r, title))
			}
		}()
		_, _ = io.Copy(c.local, c.remote)
	}()
	_, _ = io.Copy(c.remote, c.local)
}

func (c *conn) Close() error {
	return c.local.Close()
}
