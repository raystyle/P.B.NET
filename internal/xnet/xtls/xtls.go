package xtls

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"

	"project/internal/xnet/light"
)

const (
	defaultDialTimeout = 30 * time.Second
)

// Conn is light.Conn
type Conn = light.Conn

// Server is used to wrap a conn to server side conn
func Server(ctx context.Context, conn net.Conn, cfg *tls.Config, timeout time.Duration) *Conn {
	return light.Server(ctx, tls.Server(conn, cfg), timeout)
}

// Client is used to wrap a conn to client side conn
func Client(ctx context.Context, conn net.Conn, cfg *tls.Config, timeout time.Duration) *Conn {
	return light.Client(ctx, tls.Client(conn, cfg), timeout)
}

// Listen is used to listen a inner listener
func Listen(
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
) (net.Listener, error) {
	listener, err := tls.Listen(network, address, config)
	if err != nil {
		return nil, err
	}
	return light.NewListener(listener, timeout), nil
}

// Dial is used to dial a connection with context.Background()
func Dial(
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
	dialContext func(context.Context, string, string) (net.Conn, error),
) (*Conn, error) {
	return DialContext(context.Background(), network, address, config, timeout, dialContext)
}

// DialContext is used to dial a connection with context
// if dialContext is nil, dialContext = new(net.Dialer).DialContext
func DialContext(
	ctx context.Context,
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
	dialContext func(context.Context, string, string) (net.Conn, error),
) (*Conn, error) {
	if timeout < 1 {
		timeout = defaultDialTimeout
	}
	if dialContext == nil {
		dialContext = new(net.Dialer).DialContext
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	rawConn, err := dialContext(ctx, network, address)
	if err != nil {
		return nil, err
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
	client := light.Client(ctx, tls.Client(rawConn, config), timeout)
	err = client.Handshake()
	if err != nil {
		return nil, err
	}
	return client, nil
}
