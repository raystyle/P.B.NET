package xtls

import (
	"crypto/tls"
	"net"
	"time"

	"project/internal/xnet/light"
)

type Conn = light.Conn

func Server(conn net.Conn, cfg *tls.Config, timeout time.Duration) *Conn {
	tlsConn := tls.Server(conn, cfg)
	return light.Server(tlsConn, timeout)
}

// should set ServerName
func Client(conn net.Conn, cfg *tls.Config, timeout time.Duration) *Conn {
	tlsConn := tls.Client(conn, cfg)
	return light.Client(tlsConn, timeout)
}

func Listen(network, address string, cfg *tls.Config, timeout time.Duration) (net.Listener, error) {
	l, err := tls.Listen(network, address, cfg)
	if err != nil {
		return nil, err
	}
	return light.NewListener(l, timeout), nil
}

func Dial(network, address string, cfg *tls.Config, timeout time.Duration) (*Conn, error) {
	conn, err := tls.Dial(network, address, cfg)
	if err != nil {
		return nil, err
	}
	return light.Client(conn, timeout), nil
}
