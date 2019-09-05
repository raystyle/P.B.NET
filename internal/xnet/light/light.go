package light

import (
	"net"
	"time"
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

/*
type timeoutError struct{}

func (timeoutError) Error() string   { return "light: Dial With Dialer timed out" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func Dial_With_Dialer(dialer *net.Dialer, network, address string) (*Conn, error) {
	timeout := dialer.Timeout
	if !dialer.Deadline.IsZero() {
		deadlineTimeout := time.Until(dialer.Deadline)
		if timeout == 0 || deadlineTimeout < timeout {
			timeout = deadlineTimeout
		}
	}
	var errChannel chan error
	if timeout != 0 {
		errChannel = make(chan error, 2)
		time.AfterFunc(timeout, func() {
			errChannel <- timeoutError{}
		})
	}
	raw_conn, err := dialer.Dial(network, address)
	if err != nil {
		return nil, err
	}
	conn := client(raw_conn, timeout)
	if timeout == 0 {
		err = conn.Handshake()
	} else {
		go func() {
			errChannel <- conn.Handshake()
		}()
		err = <-errChannel
	}
	if err != nil {
		_ = raw_conn.Close()
		return nil, err
	}
	return conn, nil
}
*/
