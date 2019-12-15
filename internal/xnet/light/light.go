package light

import (
	"context"
	"net"
	"time"
)

const defaultDialTimeout = 30 * time.Second

// Server is used to wrap a conn to server side conn
func Server(ctx context.Context, conn net.Conn, timeout time.Duration) *Conn {
	return &Conn{ctx: ctx, Conn: conn, handshakeTimeout: timeout}
}

// Client is used to wrap a conn to client side conn
func Client(ctx context.Context, conn net.Conn, timeout time.Duration) *Conn {
	return &Conn{ctx: ctx, Conn: conn, handshakeTimeout: timeout, isClient: true}
}

type listener struct {
	net.Listener
	timeout time.Duration // handshake timeout
	ctx     context.Context
	cancel  context.CancelFunc
}

func (l *listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return Server(l.ctx, conn, l.timeout), nil
}

func (l *listener) Close() error {
	l.cancel()
	return l.Listener.Close()
}

// Listen is used to listen a inner listener
func Listen(network, address string, timeout time.Duration) (net.Listener, error) {
	l, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return NewListener(l, timeout), nil
}

// NewListener creates a Listener which accepts connections from an inner
func NewListener(inner net.Listener, timeout time.Duration) net.Listener {
	l := listener{
		Listener: inner,
		timeout:  timeout,
	}
	l.ctx, l.cancel = context.WithCancel(context.Background())
	return &l
}

// Dial is used to dial a connection with context.Background()
func Dial(
	network string,
	address string,
	timeout time.Duration,
	dialContext func(context.Context, string, string) (net.Conn, error),
) (*Conn, error) {
	return DialContext(context.Background(), network, address, timeout, dialContext)
}

// DialContext is used to dial a connection with context
// if dialContext is nil, dialContext = new(net.Dialer).DialContext
func DialContext(
	ctx context.Context,
	network string,
	address string,
	timeout time.Duration,
	dialContext func(context.Context, string, string) (net.Conn, error),
) (*Conn, error) {
	if timeout < 1 {
		timeout = defaultDialTimeout
	}
	if dialContext == nil {
		dialContext = new(net.Dialer).DialContext
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := dialContext(dialCtx, network, address)
	if err != nil {
		return nil, err
	}
	return Client(ctx, conn, timeout), nil
}
