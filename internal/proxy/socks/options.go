package socks

import (
	"strings"
	"time"

	"github.com/pkg/errors"

	"project/internal/nettool"
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
	DialContext nettool.DialContext `toml:"-" msgpack:"-"`
}

// CheckNetworkAndAddress is used to check network is supported and address is valid.
func CheckNetworkAndAddress(network, address string) error {
	switch network {
	case "tcp", "tcp4", "tcp6",
		"udp", "udp4", "udp6": // udp is not implemented.
	default:
		return errors.Errorf("unsupported network: %s", network)
	}
	if !strings.Contains(address, ":") {
		return errors.New("missing port in address")
	}
	return nil
}
