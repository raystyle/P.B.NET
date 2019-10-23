package socks5

import (
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/options"
	"project/internal/xnet/xnetutil"
	"project/internal/xpanic"
)

type Options struct {
	Username string
	Password string
	MaxConns int
	Timeout  time.Duration // handshake timeout
	ExitFunc func()
}

type Server struct {
	tag      string
	logger   logger.Logger
	maxConns int
	timeout  time.Duration
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
	server := &Server{
		tag:      "socks5-" + tag,
		logger:   lg,
		timeout:  opts.Timeout,
		maxConns: opts.MaxConns,
		conns:    make(map[string]*conn),
	}
	if opts.Username != "" {
		server.username = []byte(opts.Username)
		server.password = []byte(opts.Password)
	}
	if server.timeout < 1 {
		server.timeout = options.DefaultHandshakeTimeout
	}
	if server.maxConns < 1 {
		server.maxConns = options.DefaultMaxConns
	}
	return server, nil
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
	s.listener = netutil.LimitListener(l, s.maxConns)
	s.rwm.Unlock()
	s.wg.Add(1)
	go func() {
		var err error
		defer func() {
			if r := recover(); r != nil {
				s.log(logger.Fatal, xpanic.Sprint(r, "Server.Serve()"))
			}
			s.closeOnce.Do(func() { err = s.listener.Close() })
			if err != nil {
				s.log(logger.Error, err)
			}
			// close all conns
			s.connsRWM.Lock()
			for _, conn := range s.conns {
				_ = conn.conn.Close()
			}
			s.connsRWM.Unlock()
			// exit func
			if s.exitFunc != nil {
				s.exitFunc()
			}
			s.log(logger.Info, "server stopped")
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
				s.wg.Add(1)
				go c.serve()
			}
		}
	}()
}

func (s *Server) Close() (err error) {
	atomic.StoreInt32(&s.inShutdown, 1)
	s.closeOnce.Do(func() { err = s.listener.Close() })
	s.wg.Wait()
	return
}

func (s *Server) Address() string {
	s.rwm.RLock()
	addr := s.address
	s.rwm.RUnlock()
	return addr
}

func (s *Server) Info() string {
	s.rwm.RLock()
	a := s.address
	u := s.username
	p := s.password
	s.rwm.RUnlock()
	return fmt.Sprintf("listen: %s auth: %s %s", a, u, p)
}

func (s *Server) log(lv logger.Level, log ...interface{}) {
	s.logger.Println(lv, s.tag, log...)
}

func (s *Server) logf(lv logger.Level, format string, log ...interface{}) {
	s.logger.Printf(lv, s.tag, format, log...)
}

func (s *Server) shuttingDown() bool {
	return atomic.LoadInt32(&s.inShutdown) != 0
}

func (s *Server) newConn(c net.Conn) *conn {
	if !s.shuttingDown() {
		conn := &conn{
			server: s,
			conn:   xnetutil.DeadlineConn(c, s.timeout),
		}
		s.connsRWM.Lock()
		s.conns[conn.key()] = conn
		s.connsRWM.Unlock()
		return conn
	}
	_ = c.Close()
	return nil
}

var (
	ReplySucceeded         = []byte{version5, succeeded, reserve, ipv4, 0, 0, 0, 0, 0, 0}
	ReplyConnectRefused    = []byte{version5, connRefused, reserve, ipv4, 0, 0, 0, 0, 0, 0}
	ReplyAddressNotSupport = []byte{version5, addressNotSupport, reserve, ipv4, 0, 0, 0, 0, 0, 0}
)

type log struct {
	Log interface{}
	C   net.Conn
}

func (l *log) String() string {
	return fmt.Sprint(l.Log, "\n", logger.Conn(l.C))
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

func (c *conn) serve() {
	var err error
	defer func() {
		if r := recover(); r != nil {
			l := &log{Log: xpanic.Sprint(r, "conn.serve()"), C: c.conn}
			c.server.log(logger.Error, l)
		}
		if err != nil {
			c.server.log(logger.Error, &log{Log: err, C: c.conn})
		}
		_ = c.conn.Close()
		c.server.connsRWM.Lock()
		delete(c.server.conns, c.key())
		c.server.connsRWM.Unlock()
		c.server.wg.Done()
	}()
	buffer := make([]byte, 16) // prepare
	// read version
	_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
	if err != nil {
		return
	}
	if buffer[0] != version5 {
		c.server.log(logger.Exploit, &log{Log: "unexpected protocol version", C: c.conn})
		return
	}
	// read authentication methods
	_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
	if err != nil {
		return
	}
	l := int(buffer[0])
	if l == 0 {
		c.server.log(logger.Exploit, &log{Log: "no authentication method", C: c.conn})
		return
	}
	if l > len(buffer) {
		buffer = make([]byte, l)
	}
	_, err = io.ReadAtLeast(c.conn, buffer[:l], l)
	if err != nil {
		return
	}
	// write authentication method
	if c.server.username != nil {
		_, err = c.conn.Write([]byte{version5, usernamePassword})
		if err != nil {
			return
		}
		// read username and password version
		_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
		if err != nil {
			return
		}
		if buffer[0] != usernamePasswordVersion {
			l := &log{Log: "unexpected username password version", C: c.conn}
			c.server.log(logger.Exploit, l)
			return
		}
		// read username length
		_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
		if err != nil {
			return
		}
		l = int(buffer[0])
		if l > len(buffer) {
			buffer = make([]byte, l)
		}
		// read username
		_, err = io.ReadAtLeast(c.conn, buffer[:l], l)
		if err != nil {
			return
		}
		username := make([]byte, l)
		copy(username, buffer[:l])
		// read password length
		_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
		if err != nil {
			return
		}
		l = int(buffer[0])
		if l > len(buffer) {
			buffer = make([]byte, l)
		}
		// read password
		_, err = io.ReadAtLeast(c.conn, buffer[:l], l)
		if err != nil {
			return
		}
		password := make([]byte, l)
		copy(password, buffer[:l])
		// write username password version
		_, err = c.conn.Write([]byte{usernamePasswordVersion})
		if err != nil {
			return
		}
		if subtle.ConstantTimeCompare(c.server.username, username) != 1 ||
			subtle.ConstantTimeCompare(c.server.password, password) != 1 {
			l := fmt.Sprintf("invalid username password: %s %s", username, password)
			c.server.log(logger.Exploit, &log{Log: l, C: c.conn})
			_, err = c.conn.Write([]byte{statusFailed})
			return
		} else {
			_, err = c.conn.Write([]byte{statusSucceeded})
			if err != nil {
				return
			}
		}
	} else {
		_, err = c.conn.Write([]byte{version5, notRequired})
		if err != nil {
			return
		}
	}
	// receive connect target
	// version | cmd | reserve | address type
	if len(buffer) < 10 {
		buffer = make([]byte, 4+net.IPv4len+2) // 4 + 4(ipv4) + 2(port)
	}
	_, err = io.ReadAtLeast(c.conn, buffer[:4], 4)
	if err != nil {
		return
	}
	if buffer[0] != version5 {
		c.server.log(logger.Exploit, &log{Log: "unexpected connect protocol version", C: c.conn})
		return
	}
	if buffer[1] != connect {
		c.server.log(logger.Exploit, &log{Log: "unknown command", C: c.conn})
		_, err = c.conn.Write([]byte{version5, commandNotSupport, reserve})
		return
	}
	if buffer[2] != reserve { // reserve
		c.server.log(logger.Exploit, &log{Log: "non-zero reserved field", C: c.conn})
		_, err = c.conn.Write([]byte{version5, noReserve, reserve})
		return
	}
	// read address
	var host string
	switch buffer[3] {
	case ipv4:
		_, err = io.ReadAtLeast(c.conn, buffer[:net.IPv4len], net.IPv4len)
		if err != nil {
			return
		}
		host = net.IP(buffer[:net.IPv4len]).String()
	case ipv6:
		buffer = make([]byte, net.IPv6len)
		_, err = io.ReadAtLeast(c.conn, buffer[:net.IPv6len], net.IPv6len)
		if err != nil {
			return
		}
		host = "[" + net.IP(buffer[:net.IPv6len]).String() + "]"
	case fqdn:
		// get FQDN length
		_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
		if err != nil {
			return
		}
		l = int(buffer[0])
		if l > len(buffer) {
			buffer = make([]byte, l)
		}
		_, err = io.ReadAtLeast(c.conn, buffer[:l], l)
		if err != nil {
			return
		}
		host = string(buffer[:l])
	default:
		c.server.log(logger.Exploit, &log{Log: "address type not supported", C: c.conn})
		_, err = c.conn.Write(ReplyAddressNotSupport)
		return
	}
	// get port
	_, err = io.ReadAtLeast(c.conn, buffer[:2], 2)
	if err != nil {
		return
	}
	// start dial
	port := convert.BytesToUint16(buffer[:2])
	address := fmt.Sprintf("%s:%d", host, port)
	c.server.log(logger.Debug, &log{Log: "connect: " + address, C: c.conn})
	var remoteConn net.Conn
	remoteConn, err = net.Dial("tcp", address)
	if err != nil {
		_, err = c.conn.Write(ReplyConnectRefused)
		return
	}
	defer func() { _ = remoteConn.Close() }()
	// write reply
	// ipv4 + 0.0.0.0 + 0(port)
	_, err = c.conn.Write(ReplySucceeded)
	if err != nil {
		return
	}
	_ = remoteConn.SetDeadline(time.Time{})
	_ = c.conn.SetDeadline(time.Time{})
	c.server.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				l := &log{Log: xpanic.Sprint(r, "conn.serve()"), C: c.conn}
				c.server.log(logger.Error, l)
			}
			c.server.wg.Done()
		}()
		_, _ = io.Copy(c.conn, remoteConn)
	}()
	_, _ = io.Copy(remoteConn, c.conn)
}
