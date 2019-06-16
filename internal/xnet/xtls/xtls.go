package xtls

import (
	"crypto/tls"
	"net"

	"project/internal/xnet/light"
)

type Conn = light.Conn

func Listen(network, address string, config *tls.Config) (net.Listener, error) {
	l, err := tls.Listen(network, address, config)
	if err != nil {
		return nil, err
	}
	return light.New_Listener(l, 0), nil
}

func Dial(network, address string, config *tls.Config) (*Conn, error) {
	conn, err := tls.Dial(network, address, config)
	if err != nil {
		return nil, err
	}
	return light.Client(conn, 0), nil
}
