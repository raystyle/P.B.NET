package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"project/internal/nettool"
	"project/internal/option"
)

const (
	defaultDialTimeout    = 30 * time.Second
	defaultConnectTimeout = 15 * time.Second
	defaultMaxConnections = 1000
)

// Options contains client and server options.
type Options struct {
	Username string        `toml:"username"`
	Password string        `toml:"password"`
	Timeout  time.Duration `toml:"timeout"`

	// only client
	Header    http.Header      `toml:"header"`
	TLSConfig option.TLSConfig `toml:"tls_config" check:"-"` // only https

	// only server
	MaxConns  int                  `toml:"max_conns"`
	Server    option.HTTPServer    `toml:"server" check:"-"`
	Transport option.HTTPTransport `toml:"transport" check:"-"`

	// secondary proxy
	DialContext nettool.DialContext `toml:"-" msgpack:"-"`
}

// CheckNetworkAndAddress is used to check network is supported and address is valid.
func CheckNetworkAndAddress(network, address string) error {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return errors.Errorf("unsupported network: %s", network)
	}
	if !strings.Contains(address, ":") {
		return errors.New("missing port in address")
	}
	return nil
}
