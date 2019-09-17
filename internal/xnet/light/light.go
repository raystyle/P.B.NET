package light

import (
	"net"
	"time"

	"project/internal/xnet/internal"
)

func Server(conn net.Conn, timeout time.Duration) *Conn {
	dc := internal.NewDeadlineConn(conn, timeout)
	return &Conn{Conn: dc, handshakeTimeout: timeout}
}

func Client(conn net.Conn, timeout time.Duration) *Conn {
	dc := internal.NewDeadlineConn(conn, timeout)
	return &Conn{Conn: dc, handshakeTimeout: timeout, isClient: true}
}

type listener struct {
	net.Listener
	timeout time.Duration // handshake timeout
}

func (l *listener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return Server(c, l.timeout), nil
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

// timeout is for handshake
func Dial(network, address string, timeout time.Duration) (*Conn, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return Client(conn, timeout), nil
}
