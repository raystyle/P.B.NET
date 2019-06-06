package socks5

import (
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/connection"
	"project/internal/convert"
	"project/internal/logger"
	"project/internal/options"
)

type Options struct {
	Username          string
	Password          string
	Handshake_Timeout time.Duration
	Limit             int
}

type Server struct {
	tag               string
	logger            logger.Logger
	listener          net.Listener
	username          []byte
	password          []byte
	handshake_timeout time.Duration
	limit             int
	conns             map[string]*conn // key = conn.addr
	rwm               sync.RWMutex     // lock conns
	addr              string
	is_stopped        bool
	m                 sync.Mutex
	stop_signal       chan struct{}
}

func New_Server(tag string, l logger.Logger, opts *Options) (*Server, error) {
	if tag == "" {
		return nil, errors.New("no tag")
	}
	if opts == nil {
		opts = new(Options)
	}
	s := &Server{
		tag:               tag,
		logger:            l,
		handshake_timeout: options.DEFAULT_HANDSHAKE_TIMEOUT,
		limit:             options.DEFAULT_CONNECTION_LIMIT,
		conns:             make(map[string]*conn),
		stop_signal:       make(chan struct{}, 1),
	}
	if opts.Username != "" {
		s.username = []byte(opts.Username)
		s.password = []byte(opts.Password)
	}
	if opts.Handshake_Timeout > 0 {
		s.handshake_timeout = opts.Handshake_Timeout
	}
	if opts.Limit > 0 {
		s.limit = opts.Limit
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
	l = connection.Limit_Listener(l, this.limit)
	this.listener = l
	// reference http.Server.Serve()
	f := func() error {
		var temp_delay time.Duration // how long to sleep on accept failure
		max := 1 * time.Second
		for {
			conn, err := l.Accept()
			if err != nil {
				select {
				case <-this.stop_signal:
					return errors.New("server closed")
				default:
				}
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					if temp_delay == 0 {
						temp_delay = 5 * time.Millisecond
					} else {
						temp_delay *= 2
					}
					if temp_delay > max {
						temp_delay = max
					}
					this.log(logger.WARNING, "Accept error: %s; retrying in %v", err, temp_delay)
					time.Sleep(temp_delay)
					continue
				}
				return err
			}
			temp_delay = 0
			c := this.new_conn(conn)
			if c != nil {
				go c.serve()
			} else {
				return errors.New("server closed")
			}
		}
	}
	return this.start(f, start_timeout)
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
	defer this.m.Unlock()
	this.m.Lock()
	this.is_stopped = true
	this.stop_signal <- struct{}{}
	err := this.listener.Close()
	for k, v := range this.conns {
		_ = v.conn.Close()
		delete(this.conns, k)
	}
	this.log(logger.INFO, "server stopped")
	return err
}

func (this *Server) Info() string {
	defer this.m.Unlock()
	this.m.Lock()
	return fmt.Sprintf("Listen: %s Auth: %s %s", this.addr, this.username, this.password)
}

func (this *Server) Addr() string {
	defer this.m.Unlock()
	this.m.Lock()
	return this.addr
}

func (this *Server) log(level logger.Level, log ...interface{}) {
	this.logger.Println(level, this.tag, log...)
}

func (this *Server) new_conn(c net.Conn) *conn {
	defer this.m.Unlock()
	this.m.Lock()
	if !this.is_stopped {
		conn := &conn{
			server: this,
			conn:   c,
		}
		this.rwm.Lock()
		this.conns[c.RemoteAddr().String()] = conn
		this.rwm.Unlock()
		return conn
	}
	return nil
}

type serve_log struct {
	Log interface{}
	C   net.Conn
}

func (this *serve_log) String() string {
	return fmt.Sprint(this.Log, " Client: ", this.C.RemoteAddr())
}

type conn struct {
	server *Server
	conn   net.Conn
}

func (this *conn) serve() {
	defer func() {
		if rec := recover(); rec != nil {
			this.server.log(logger.ERROR, &serve_log{Log: fmt.Sprint("panic: ", rec), C: this.conn})
		}
		_ = this.conn.Close()
		this.server.rwm.Lock()
		delete(this.server.conns, this.conn.RemoteAddr().String())
		this.server.rwm.Unlock()
	}()
	// set handshake timeout
	_ = this.conn.SetDeadline(time.Now().Add(this.server.handshake_timeout))
	buffer := make([]byte, 16)
	// read version
	_, err := io.ReadAtLeast(this.conn, buffer[:1], 1)
	if err != nil {
		this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
		return
	}
	if buffer[0] != version5 {
		this.server.log(logger.EXPLOIT, &serve_log{C: this.conn,
			Log: fmt.Sprintf("unexpected protocol version %d", buffer[0])})
		return
	}
	// read authentication methods
	_, err = io.ReadAtLeast(this.conn, buffer[:1], 1)
	if err != nil {
		this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
		return
	}
	l := int(buffer[0])
	if l == 0 {
		this.server.log(logger.EXPLOIT, &serve_log{C: this.conn,
			Log: "unexpected authentication method length 0"})
		return
	}
	if l > len(buffer) {
		buffer = make([]byte, l)
	}
	_, err = io.ReadAtLeast(this.conn, buffer[:l], l)
	if err != nil {
		this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
		return
	}
	// write authentication method
	if this.server.username != nil {
		_, err = this.conn.Write([]byte{version5, username_password})
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
			return
		}
		// read username and password version
		_, err = io.ReadAtLeast(this.conn, buffer[:1], 1)
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
			return
		}
		if buffer[0] != username_password_version {
			this.server.log(logger.EXPLOIT, &serve_log{C: this.conn,
				Log: fmt.Sprintf("unexpected username password version %d", buffer[0])})
			return
		}
		// read username length
		_, err = io.ReadAtLeast(this.conn, buffer[:1], 1)
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
			return
		}
		l = int(buffer[0])
		if l > len(buffer) {
			buffer = make([]byte, l)
		}
		// read username
		_, err = io.ReadAtLeast(this.conn, buffer[:l], l)
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
			return
		}
		username := make([]byte, l)
		copy(username, buffer[:l])
		// read password length
		_, err = io.ReadAtLeast(this.conn, buffer[:1], 1)
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
			return
		}
		l = int(buffer[0])
		if l > len(buffer) {
			buffer = make([]byte, l)
		}
		// read password
		_, err = io.ReadAtLeast(this.conn, buffer[:l], l)
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
			return
		}
		password := make([]byte, l)
		copy(password, buffer[:l])
		// write username password version
		_, err = this.conn.Write([]byte{username_password_version})
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
			return
		}
		if subtle.ConstantTimeCompare(this.server.username, username) != 1 ||
			subtle.ConstantTimeCompare(this.server.password, password) != 1 {
			this.server.log(logger.EXPLOIT, &serve_log{C: this.conn,
				Log: fmt.Sprintf("invalid username password: %s %s", username, password)})
			_, err = this.conn.Write([]byte{status_failed})
			if err != nil {
				this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
			}
			return
		} else {
			_, err = this.conn.Write([]byte{status_succeeded})
			if err != nil {
				this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
				return
			}
		}
	} else {
		_, err = this.conn.Write([]byte{version5, not_required})
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
			return
		}
	}
	// receive connect target
	// version | cmd | reserve | address type
	if len(buffer) < 10 {
		buffer = make([]byte, 10) // 4 + 4(ipv4) + 2(port)
	}
	_, err = io.ReadAtLeast(this.conn, buffer[:4], 4)
	if err != nil {
		this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
		return
	}
	if buffer[0] != version5 {
		this.server.log(logger.EXPLOIT, &serve_log{C: this.conn,
			Log: fmt.Sprintf("unexpected connect protocol version %d", buffer[0])})
		return
	}
	if buffer[1] != connect {
		this.server.log(logger.EXPLOIT, &serve_log{C: this.conn,
			Log: "non-zero reserved field"})
		_, err = this.conn.Write([]byte{version5, command_not_support, reserve})
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
		}
		return
	}
	if buffer[2] != reserve { // reserve
		this.server.log(logger.EXPLOIT, &serve_log{C: this.conn, Log: "non-zero reserved field"})
		_, err = this.conn.Write([]byte{version5, 0x01, reserve})
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
		}
		return
	}
	// read address
	var host string
	switch buffer[3] {
	case ipv4:
		_, err = io.ReadAtLeast(this.conn, buffer[:4], 4)
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
			return
		}
		host = net.IP(buffer[:4]).String()
	case ipv6:
		buffer = make([]byte, 16) // 4 + 4(ipv4) + 2(port)
		_, err = io.ReadAtLeast(this.conn, buffer[:16], 16)
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
			return
		}
		host = "[" + net.IP(buffer[:16]).String() + "]"
	case fqdn:
		// get FQDN length
		_, err = io.ReadAtLeast(this.conn, buffer[:1], 1)
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
			return
		}
		l = int(buffer[0])
		if l > len(buffer) {
			buffer = make([]byte, l)
		}
		_, err = io.ReadAtLeast(this.conn, buffer[:l], l)
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
			return
		}
		host = string(buffer[:l])
	default:
		this.server.log(logger.EXPLOIT, &serve_log{C: this.conn, Log: "address type not supported"})
		_, err = this.conn.Write([]byte{version5, 0x08, reserve, ipv4, 0, 0, 0, 0, 0, 0})
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
		}
		return
	}
	// get port
	_, err = io.ReadAtLeast(this.conn, buffer[:2], 2)
	if err != nil {
		this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
		return
	}
	// start dial
	port := convert.Bytes_Uint16(buffer[:2])
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
		_, err = this.conn.Write([]byte{version5, 0x05, reserve, ipv4, 0, 0, 0, 0, 0, 0})
		if err != nil {
			this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
		}
		return
	}
	defer func() { _ = conn.Close() }()
	// write reply
	//ipv4 + 0.0.0.0 + 0(port)
	success := []byte{version5, succeeded, reserve, ipv4, 0, 0, 0, 0, 0, 0}
	_, err = this.conn.Write(success)
	if err != nil {
		this.server.log(logger.ERROR, &serve_log{Log: err, C: this.conn})
		return
	}
	_ = this.conn.SetDeadline(time.Time{})
	go func() { _, _ = io.Copy(conn, this.conn) }()
	_, _ = io.Copy(this.conn, conn)
}
