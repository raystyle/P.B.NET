package quic

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go"
)

const (
	defaultDialTimeout = 30 * time.Second
	defaultNextProto   = "h3-23" // HTTP/3
)

// Conn implement net.Conn
type Conn struct {
	ctx     context.Context
	session quic.Session
	send    quic.SendStream
	receive quic.ReceiveStream

	// must use extra Mutex because SendStream
	// is not safe for use by multiple goroutines
	m sync.Mutex
}

func newConn(ctx context.Context, session quic.Session) (*Conn, error) {
	send, err := session.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return &Conn{
		ctx:     ctx,
		session: session,
		send:    send,
	}, nil
}

func (c *Conn) acceptUniStream() error {
	c.m.Lock()
	defer c.m.Unlock()
	if c.receive == nil {
		receive, err := c.session.AcceptUniStream(c.ctx)
		if err != nil {
			return err
		}
		c.receive = receive
	}
	return nil
}

// Read reads data from the connection
func (c *Conn) Read(b []byte) (n int, err error) {
	err = c.acceptUniStream()
	if err != nil {
		return
	}
	return c.receive.Read(b)
}

// Write writes data to the connection
func (c *Conn) Write(b []byte) (n int, err error) {
	c.m.Lock()
	defer c.m.Unlock()
	return c.send.Write(b)
}

// Close is used to close connection
func (c *Conn) Close() error {
	return c.session.Close()
}

// LocalAddr is used to get local address
func (c *Conn) LocalAddr() net.Addr {
	return c.session.LocalAddr()
}

// RemoteAddr is used to get remote address
func (c *Conn) RemoteAddr() net.Addr {
	return c.session.RemoteAddr()
}

// SetDeadline is used to set read and write deadline
func (c *Conn) SetDeadline(t time.Time) error {
	err := c.SetReadDeadline(t)
	if err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

// SetReadDeadline is used to set read deadline
func (c *Conn) SetReadDeadline(t time.Time) error {
	err := c.acceptUniStream()
	if err != nil {
		return err
	}
	return c.receive.SetReadDeadline(t)
}

// SetWriteDeadline is used to set write deadline
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.send.SetWriteDeadline(t)
}

type listener struct {
	ctx    context.Context
	cancel context.CancelFunc
	quic.Listener
}

func (l *listener) Accept() (net.Conn, error) {
	session, err := l.Listener.Accept(l.ctx)
	if err != nil {
		return nil, err
	}
	return newConn(l.ctx, session)
}

func (l *listener) Close() error {
	l.cancel()
	return l.Listener.Close()
}

// Listen is used to create a listener
func Listen(
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
) (net.Listener, error) {
	addr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP(network, addr)
	if err != nil {
		return nil, err
	}
	quicCfg := quic.Config{
		HandshakeTimeout: timeout,
		IdleTimeout:      timeout,
		KeepAlive:        true,
	}
	if len(config.NextProtos) == 0 {
		config.NextProtos = []string{defaultNextProto}
	}
	l, err := quic.Listen(conn, config, &quicCfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	ll := listener{Listener: l}
	ll.ctx, ll.cancel = context.WithCancel(context.Background())
	return &ll, nil
}

// Dial is used to dial a connection with context.Background()
func Dial(
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
) (*Conn, error) {
	return DialContext(context.Background(), network, address, config, timeout)
}

// DialContext is used to dial a connection with context
func DialContext(
	ctx context.Context,
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
) (*Conn, error) {
	rAddr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP(network, nil)
	if err != nil {
		return nil, err
	}
	if timeout < 1 {
		timeout = defaultDialTimeout
	}
	quicCfg := quic.Config{
		HandshakeTimeout: timeout,
		IdleTimeout:      5 * timeout,
		KeepAlive:        true,
	}
	if len(config.NextProtos) == 0 {
		config.NextProtos = []string{defaultNextProto}
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	session, err := quic.DialContext(dialCtx, conn, rAddr, address, config, &quicCfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return newConn(ctx, session)
}
