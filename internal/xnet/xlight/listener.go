package xlight

import (
	"net"
	"time"
)

type listener struct {
	net.Listener
	handshake_timeout time.Duration
}

func (this *listener) Accept() (net.Conn, error) {
	c, err := this.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return Server(c, this.handshake_timeout), nil
}

func Listen(network, address string, timeout time.Duration) (net.Listener, error) {
	l, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return New_Listener(l, timeout), nil
}

func New_Listener(inner net.Listener, timeout time.Duration) net.Listener {
	return &listener{
		Listener:          inner,
		handshake_timeout: timeout,
	}
}
