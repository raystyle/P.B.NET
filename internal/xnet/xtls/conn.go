package xtls

import (
	"crypto/tls"

	"project/internal/xnet/light"
)

func Dial(network, address string, config *tls.Config) (*Conn, error) {
	conn, err := tls.Dial(network, address, config)
	if err != nil {
		return nil, err
	}
}

type Conn struct {
	*light.Conn
}
