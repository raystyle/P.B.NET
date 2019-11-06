package xtls

import (
	"crypto/tls"
	"net"
	"strings"
	"time"

	"project/internal/options"
	"project/internal/xnet/light"
)

const (
	defaultNextProto = "http/1.1"
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

func Listen(
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
) (net.Listener, error) {
	if len(config.NextProtos) == 0 {
		config.NextProtos = []string{defaultNextProto}
	}
	tl, err := tls.Listen(network, address, config)
	if err != nil {
		return nil, err
	}
	return light.NewListener(tl, timeout), nil
}

func Dial(
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
	dial func(string, string, time.Duration) (net.Conn, error),
) (*Conn, error) {
	if timeout < 1 {
		timeout = options.DefaultHandshakeTimeout
	}
	if dial == nil {
		dial = net.DialTimeout
	}
	rawConn, err := dial(network, address, timeout)
	if err != nil {
		return nil, err
	}
	if len(config.NextProtos) == 0 {
		config.NextProtos = []string{defaultNextProto}
	}
	if config.ServerName == "" {
		colonPos := strings.LastIndex(address, ":")
		if colonPos == -1 {
			colonPos = len(address)
		}
		hostname := address[:colonPos]
		c := config.Clone()
		c.ServerName = hostname
		config = c
	}
	return light.Client(tls.Client(rawConn, config), timeout), nil
}
