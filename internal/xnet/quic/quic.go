package quic

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go"
)

const (
	defaultTimeout   = 30 * time.Second // dial and accept
	defaultNextProto = "h3-24"          // HTTP/3
)

// ErrConnClosed is an error about closed
var ErrConnClosed = errors.New("connection closed")

// Conn implement net.Conn
type Conn struct {
	session quic.Session
	stream  quic.Stream

	// must use extra Mutex because SendStream
	// is not safe for use by multiple goroutines
	//
	// stream.Close() must not be called concurrently with Write()
	sendMutex sync.Mutex

	// only server connection need it
	timeout    time.Duration
	ctx        context.Context
	cancel     context.CancelFunc
	acceptErr  error
	acceptOnce sync.Once
}

func (c *Conn) acceptStream() error {
	c.acceptOnce.Do(func() {
		if c.stream == nil {
			defer c.cancel()
			c.stream, c.acceptErr = c.session.AcceptStream(c.ctx)
			if c.acceptErr != nil {
				return
			}
			_ = c.stream.SetReadDeadline(time.Now().Add(c.timeout))
			_, c.acceptErr = c.stream.Read(make([]byte, 1))
		}
	})
	return c.acceptErr
}

// Read reads data from the connection
func (c *Conn) Read(b []byte) (n int, err error) {
	err = c.acceptStream()
	if err != nil {
		return
	}
	return c.stream.Read(b)
}

// Write writes data to the connection
func (c *Conn) Write(b []byte) (n int, err error) {
	err = c.acceptStream()
	if err != nil {
		return
	}
	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()
	return c.stream.Write(b)
}

// Close is used to close connection
func (c *Conn) Close() error {
	c.acceptOnce.Do(func() {
		c.acceptErr = ErrConnClosed
	})
	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()
	if c.stream != nil {
		_ = c.stream.Close()
	}
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
	err := c.acceptStream()
	if err != nil {
		return err
	}
	return c.stream.SetReadDeadline(t)
}

// SetWriteDeadline is used to set write deadline
func (c *Conn) SetWriteDeadline(t time.Time) error {
	err := c.acceptStream()
	if err != nil {
		return err
	}
	return c.stream.SetWriteDeadline(t)
}

type listener struct {
	quic.Listener
	timeout time.Duration

	ctx    context.Context
	cancel context.CancelFunc
}

func (l *listener) Accept() (net.Conn, error) {
	session, err := l.Listener.Accept(l.ctx)
	if err != nil {
		return nil, err
	}
	conn := Conn{
		session: session,
		timeout: l.timeout,
	}
	conn.ctx, conn.cancel = context.WithTimeout(l.ctx, l.timeout)
	return &conn, nil
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
	if timeout < 1 {
		timeout = defaultTimeout
	}
	quicCfg := quic.Config{
		HandshakeTimeout: timeout,
		IdleTimeout:      timeout,
		KeepAlive:        true,
	}
	if len(config.NextProtos) == 0 {
		config.NextProtos = []string{defaultNextProto}
	}
	quicListener, err := quic.Listen(conn, config, &quicCfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	l := listener{
		Listener: quicListener,
		timeout:  timeout,
	}
	l.ctx, l.cancel = context.WithCancel(context.Background())
	return &l, nil
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
		timeout = defaultTimeout
	}
	quicCfg := quic.Config{
		HandshakeTimeout: timeout,
		IdleTimeout:      5 * timeout,
		KeepAlive:        true,
	}
	if len(config.NextProtos) == 0 {
		config.NextProtos = []string{defaultNextProto}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	session, err := quic.DialContext(ctx, conn, rAddr, address, config, &quicCfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	stream, err := session.OpenStreamSync(ctx)
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	_ = stream.SetWriteDeadline(time.Now().Add(timeout))
	_, err = stream.Write([]byte{0})
	if err != nil {
		_ = stream.Close()
		_ = session.Close()
		return nil, err
	}
	return &Conn{session: session, stream: stream}, nil
}
