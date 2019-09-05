package xtls

import (
	"crypto/tls"
	"net"
	"time"

	"project/internal/xnet/light"
)

type Conn = light.Conn

func Listen(network, address string, c *tls.Config, timeout time.Duration) (net.Listener, error) {
	l, err := tls.Listen(network, address, c)
	if err != nil {
		return nil, err
	}
	return light.NewListener(l, timeout), nil
}

func Dial(network, address string, c *tls.Config, timeout time.Duration) (*Conn, error) {
	conn, err := tls.Dial(network, address, c)
	if err != nil {
		return nil, err
	}
	return light.Client(conn, timeout), nil
}
