package socks

import (
	"context"
	"net"
	"time"

	"github.com/pkg/errors"
)

const (
	defaultDialTimeout    = 30 * time.Second
	defaultConnectTimeout = 15 * time.Second
	defaultMaxConnections = 1000
)

// Options contains client and server options.
type Options struct {
	// only socks5
	Username string `toml:"username"`
	Password string `toml:"password"`

	// only socks4 socks4a
	UserID string `toml:"user_id"`

	// server handshake & client dial timeout
	Timeout time.Duration `toml:"timeout"`

	// only server
	MaxConns int `toml:"max_conns"`

	// secondary proxy
	// internal/proxy.client.DialContext()
	DialContext func(ctx context.Context, network, address string) (net.Conn, error) `toml:"-"`
}

// CheckNetwork is used to check network is supported.
func CheckNetwork(network string) error {
	switch network {
	case "tcp", "tcp4", "tcp6":
		return nil
	default:
		return errors.Errorf("unsupported network: %s", network)
	}
}
