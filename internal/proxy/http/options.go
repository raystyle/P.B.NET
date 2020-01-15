package http

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"project/internal/options"
)

// Options contains client and server options
type Options struct {
	HTTPS    bool          `toml:"https"`
	Username string        `toml:"username"`
	Password string        `toml:"password"`
	Timeout  time.Duration `toml:"timeout"`

	// only client
	Header    http.Header       `toml:"header"`
	TLSConfig options.TLSConfig `toml:"tls_config"` // https

	// only server
	MaxConns  int                   `toml:"max_conns"`
	Server    options.HTTPServer    `toml:"server"`
	Transport options.HTTPTransport `toml:"transport"`

	DialContext func(ctx context.Context, network, address string) (net.Conn, error) `toml:"-"`
	ExitFunc    func()                                                               `toml:"-"`
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
