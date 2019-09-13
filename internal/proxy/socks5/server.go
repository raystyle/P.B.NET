package socks5

import (
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/options"
	"project/internal/xnet"
)

type Options struct {
	Username string
	Password string
	Timeout  time.Duration // handshake timeout
	Limit    int
}

type Server struct {
	tag        string
	logger     logger.Logger
	listener   net.Listener
	username   []byte
	password   []byte
	timeout    time.Duration
	limit      int
	conns      map[string]*conn // key = conn.addr
	rwm        sync.RWMutex     // lock conns
	addr       string
	isStopped  bool
	m          sync.Mutex
	stopSignal chan struct{}
}

func NewServer(tag string, l logger.Logger, opts *Options) (*Server, error) {
	if tag == "" {
		return nil, errors.New("no tag")
	}
	if opts == nil {
		opts = new(Options)
	}
	s := &Server{
		tag:        tag,
		logger:     l,
		timeout:    opts.Timeout,
		limit:      opts.Limit,
		conns:      make(map[string]*conn),
		stopSignal: make(chan struct{}, 1),
	}
	if opts.Username != "" {
		s.username = []byte(opts.Username)
		s.password = []byte(opts.Password)
	}
	if opts.Timeout < 1 {
		s.timeout = options.DefaultHandshakeTimeout
	}
	if opts.Limit < 1 {
		s.limit = options.DefaultConnectionLimit
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
	l = netutil.LimitListener(l, s.limit)
	s.listener = l
	// reference http.Server.Serve()
	f := func() error {
		var delay time.Duration // how long to sleep on accept failure
		max := 1 * time.Second
		for {
			conn, err := l.Accept()
			if err != nil {
				select {
				case <-s.stopSignal:
					return errors.New("server closed")
				default:
				}
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					if delay == 0 {
						delay = 5 * time.Millisecond
					} else {
						delay *= 2
					}
					if delay > max {
						delay = max
					}
					s.logf(logger.Warning, "accept error: %s; retrying in %v", err, delay)
					time.Sleep(delay)
					continue
				}
				return err
			}
			delay = 0
			c := s.newConn(conn)
			if c != nil {
				go c.serve()
			} else {
				return errors.New("server closed")
			}
		}
	}
	return s.start(f, timeout)
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
					err = errors.WithStack(v)
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
	s.m.Lock()
	defer s.m.Unlock()
	s.isStopped = true
	s.stopSignal <- struct{}{}
	err := s.listener.Close()
	s.rwm.Lock()
	for k, v := range s.conns {
		_ = v.conn.Close()
		delete(s.conns, k)
	}
	s.rwm.Unlock()
	s.log(logger.Info, "server stopped")
	return err
}

func (s *Server) Info() string {
	s.m.Lock()
	a := s.addr
	u := s.username
	p := s.password
	s.m.Unlock()
	return fmt.Sprintf("Listen: %s Auth: %s %s", a, u, p)
}

func (s *Server) Addr() string {
	s.m.Lock()
	defer s.m.Unlock()
	return s.addr
}

func (s *Server) log(l logger.Level, log ...interface{}) {
	s.logger.Println(l, s.tag, log...)
}

func (s *Server) logf(l logger.Level, format string, log ...interface{}) {
	s.logger.Printf(l, s.tag, format, log...)
}

func (s *Server) newConn(c net.Conn) *conn {
	s.m.Lock()
	defer s.m.Unlock()
	if !s.isStopped {
		conn := &conn{
			server: s,
			conn:   xnet.NewDeadlineConn(c, s.timeout),
		}
		s.rwm.Lock()
		s.conns[c.RemoteAddr().String()] = conn
		s.rwm.Unlock()
		return conn
	}
	return nil
}

type log struct {
	Log interface{}
	C   net.Conn
}

func (l *log) String() string {
	return fmt.Sprint(l.Log, " client: ", l.C.RemoteAddr())
}

type conn struct {
	server *Server
	conn   net.Conn
}

var (
	ReplySucceeded         = []byte{version5, succeeded, reserve, ipv4, 0, 0, 0, 0, 0, 0}
	ReplyConnectRefused    = []byte{version5, connRefused, reserve, ipv4, 0, 0, 0, 0, 0, 0}
	ReplyAddressNotSupport = []byte{version5, addressNotSupport, reserve, ipv4, 0, 0, 0, 0, 0, 0}
)

func (c *conn) serve() {
	defer func() {
		if rec := recover(); rec != nil {
			c.server.log(logger.Error, &log{Log: fmt.Sprint("panic: ", rec), C: c.conn})
		}
		_ = c.conn.Close()
		c.server.rwm.Lock()
		delete(c.server.conns, c.conn.RemoteAddr().String())
		c.server.rwm.Unlock()
	}()
	buffer := make([]byte, 16)
	// read version
	_, err := io.ReadAtLeast(c.conn, buffer[:1], 1)
	if err != nil {
		c.server.log(logger.Error, &log{Log: err, C: c.conn})
		return
	}
	if buffer[0] != version5 {
		c.server.log(logger.Exploit, &log{C: c.conn,
			Log: fmt.Sprintf("unexpected protocol version %d", buffer[0])})
		return
	}
	// read authentication methods
	_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
	if err != nil {
		c.server.log(logger.Error, &log{Log: err, C: c.conn})
		return
	}
	l := int(buffer[0])
	if l == 0 {
		c.server.log(logger.Exploit, &log{C: c.conn,
			Log: "unexpected authentication method length 0"})
		return
	}
	if l > len(buffer) {
		buffer = make([]byte, l)
	}
	_, err = io.ReadAtLeast(c.conn, buffer[:l], l)
	if err != nil {
		c.server.log(logger.Error, &log{Log: err, C: c.conn})
		return
	}
	// write authentication method
	if c.server.username != nil {
		_, err = c.conn.Write([]byte{version5, usernamePassword})
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
			return
		}
		// read username and password version
		_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
			return
		}
		if buffer[0] != usernamePasswordVersion {
			c.server.log(logger.Exploit, &log{C: c.conn,
				Log: fmt.Sprintf("unexpected username password version %d", buffer[0])})
			return
		}
		// read username length
		_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
			return
		}
		l = int(buffer[0])
		if l > len(buffer) {
			buffer = make([]byte, l)
		}
		// read username
		_, err = io.ReadAtLeast(c.conn, buffer[:l], l)
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
			return
		}
		username := make([]byte, l)
		copy(username, buffer[:l])
		// read password length
		_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
			return
		}
		l = int(buffer[0])
		if l > len(buffer) {
			buffer = make([]byte, l)
		}
		// read password
		_, err = io.ReadAtLeast(c.conn, buffer[:l], l)
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
			return
		}
		password := make([]byte, l)
		copy(password, buffer[:l])
		// write username password version
		_, err = c.conn.Write([]byte{usernamePasswordVersion})
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
			return
		}
		if subtle.ConstantTimeCompare(c.server.username, username) != 1 ||
			subtle.ConstantTimeCompare(c.server.password, password) != 1 {
			c.server.log(logger.Exploit, &log{C: c.conn,
				Log: fmt.Sprintf("invalid username password: %s %s", username, password)})
			_, err = c.conn.Write([]byte{statusFailed})
			if err != nil {
				c.server.log(logger.Error, &log{Log: err, C: c.conn})
			}
			return
		} else {
			_, err = c.conn.Write([]byte{statusSucceeded})
			if err != nil {
				c.server.log(logger.Error, &log{Log: err, C: c.conn})
				return
			}
		}
	} else {
		_, err = c.conn.Write([]byte{version5, notRequired})
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
			return
		}
	}
	// receive connect target
	// version | cmd | reserve | address type
	if len(buffer) < 10 {
		buffer = make([]byte, 10) // 4 + 4(ipv4) + 2(port)
	}
	_, err = io.ReadAtLeast(c.conn, buffer[:4], 4)
	if err != nil {
		c.server.log(logger.Error, &log{Log: err, C: c.conn})
		return
	}
	if buffer[0] != version5 {
		c.server.log(logger.Exploit, &log{C: c.conn,
			Log: fmt.Sprintf("unexpected connect protocol version %d", buffer[0])})
		return
	}
	if buffer[1] != connect {
		c.server.log(logger.Exploit, &log{C: c.conn,
			Log: "non-zero reserved field"})
		_, err = c.conn.Write([]byte{version5, commandNotSupport, reserve})
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
		}
		return
	}
	if buffer[2] != reserve { // reserve
		c.server.log(logger.Exploit, &log{C: c.conn, Log: "non-zero reserved field"})
		_, err = c.conn.Write([]byte{version5, 0x01, reserve})
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
		}
		return
	}
	// read address
	var host string
	switch buffer[3] {
	case ipv4:
		_, err = io.ReadAtLeast(c.conn, buffer[:4], 4)
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
			return
		}
		host = net.IP(buffer[:4]).String()
	case ipv6:
		buffer = make([]byte, 16) // 4 + 4(ipv4) + 2(port)
		_, err = io.ReadAtLeast(c.conn, buffer[:16], 16)
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
			return
		}
		host = "[" + net.IP(buffer[:16]).String() + "]"
	case fqdn:
		// get FQDN length
		_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
			return
		}
		l = int(buffer[0])
		if l > len(buffer) {
			buffer = make([]byte, l)
		}
		_, err = io.ReadAtLeast(c.conn, buffer[:l], l)
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
			return
		}
		host = string(buffer[:l])
	default:
		c.server.log(logger.Exploit, &log{C: c.conn, Log: "address type not supported"})
		_, err = c.conn.Write(ReplyAddressNotSupport)
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
		}
		return
	}
	// get port
	_, err = io.ReadAtLeast(c.conn, buffer[:2], 2)
	if err != nil {
		c.server.log(logger.Error, &log{Log: err, C: c.conn})
		return
	}
	// start dial
	port := convert.BytesToUint16(buffer[:2])
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		c.server.log(logger.Error, &log{Log: err, C: c.conn})
		_, err = c.conn.Write(ReplyConnectRefused)
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
		}
		return
	}
	defer func() { _ = conn.Close() }()
	// write reply
	// ipv4 + 0.0.0.0 + 0(port)
	_, err = c.conn.Write(ReplySucceeded)
	if err != nil {
		c.server.log(logger.Error, &log{Log: err, C: c.conn})
		return
	}
	_ = c.conn.SetDeadline(time.Time{})
	go func() { _, _ = io.Copy(conn, c.conn) }()
	_, _ = io.Copy(c.conn, conn)
}
