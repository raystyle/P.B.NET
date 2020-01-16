package http

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"project/internal/option"
)

// Options contains client and server options
type Options struct {
	Username string        `toml:"username"`
	Password string        `toml:"password"`
	Timeout  time.Duration `toml:"timeout"`

	// only client
	Header    http.Header      `toml:"header"`
	TLSConfig option.TLSConfig `toml:"tls_config"` // only https

	// only server
	MaxConns  int                  `toml:"max_conns"`
	Server    option.HTTPServer    `toml:"server"`
	Transport option.HTTPTransport `toml:"transport"`

	DialContext func(ctx context.Context, network, address string) (net.Conn, error) `toml:"-"`
}

// CheckNetwork is used to check network is supported
func CheckNetwork(network string) error {
	switch network {
	case "tcp", "tcp4", "tcp6":
		return nil
	default:
		return errors.Errorf("unsupported network: %s", network)
	}
}
