package xtls

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"

	"project/internal/options"
	"project/internal/xnet/light"
)

const (
	defaultNextProto = "http/1.1"
)

type Conn = light.Conn

func Server(ctx context.Context, conn net.Conn, cfg *tls.Config, timeout time.Duration) *Conn {
	tlsConn := tls.Server(conn, cfg)
	return light.Server(ctx, tlsConn, timeout)
}

// should set ServerName
func Client(ctx context.Context, conn net.Conn, cfg *tls.Config, timeout time.Duration) *Conn {
	tlsConn := tls.Client(conn, cfg)
	return light.Client(ctx, tlsConn, timeout)
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
	dialContext func(context.Context, string, string) (net.Conn, error),
) (*Conn, error) {
	return DialContext(context.Background(), network, address, config, timeout, dialContext)
}

func DialContext(
	ctx context.Context,
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
	dialContext func(context.Context, string, string) (net.Conn, error),
) (*Conn, error) {
	if timeout < 1 {
		timeout = options.DefaultHandshakeTimeout
	}
	if dialContext == nil {
		dialContext = new(net.Dialer).DialContext
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	rawConn, err := dialContext(dialCtx, network, address)
	if err != nil {
		return nil, err
	}
	if len(config.NextProtos) == 0 {
		config.NextProtos = []string{defaultNextProto}
	}
	if config.ServerName == "" {
		colonPos := strings.LastIndex(address, ":")
		if colonPos == -1 {
			return nil, errors.New("missing port in address")
		}
		hostname := address[:colonPos]
		c := config.Clone()
		c.ServerName = hostname
		config = c
	}
	return light.Client(ctx, tls.Client(rawConn, config), timeout), nil
}
