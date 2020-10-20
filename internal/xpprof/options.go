package xpprof

import (
	"time"

	"github.com/pkg/errors"

	"project/internal/option"
)

const (
	defaultTimeout        = 15 * time.Second
	defaultMaxConnections = 1000
)

// Options contains options about pprof server.
type Options struct {
	Username string        `toml:"username"`
	Password string        `toml:"password"`
	Timeout  time.Duration `toml:"timeout"`

	MaxConns int               `toml:"max_conns"`
	Server   option.HTTPServer `toml:"server" check:"-"`
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
