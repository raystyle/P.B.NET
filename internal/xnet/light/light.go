package light

import (
	"net"
	"time"

	"project/internal/options"
	"project/internal/proxy/direct"
	"project/internal/xnet/internal"
)

func Server(conn net.Conn, timeout time.Duration) *Conn {
	return &Conn{Conn: conn, handshakeTimeout: timeout}
}

func Client(conn net.Conn, timeout time.Duration) *Conn {
	return &Conn{Conn: conn, handshakeTimeout: timeout, isClient: true}
}

type listener struct {
	net.Listener
	timeout time.Duration // handshake timeout
}

func (l *listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return Server(conn, l.timeout), nil
}

func Listen(network, address string, timeout time.Duration) (net.Listener, error) {
	l, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return NewListener(l, timeout), nil
}

func NewListener(inner net.Listener, timeout time.Duration) net.Listener {
	return &listener{
		Listener: inner,
		timeout:  timeout,
	}
}

func Dial(
	network string,
	address string,
	timeout time.Duration,
	dialer internal.Dialer,
) (*Conn, error) {
	if timeout < 1 {
		timeout = options.DefaultDialTimeout
	}
	if dialer == nil {
		dialer = new(direct.Direct)
	}
	conn, err := dialer.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, err
	}
	return Client(conn, timeout), nil
}
