package bootstrap

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"project/internal/dns"
	"project/internal/proxy"
)

// supported modes
const (
	ModeHTTP   = "http"
	ModeDNS    = "dns"
	ModeDirect = "direct"
)

// Listener is the bootstrap node listener
// Node or Beacon register will use bootstrap to resolve node listeners
// you can reference internal/xnet/net.go
type Listener struct {
	Mode    string `toml:"mode"`
	Network string `toml:"network"`
	Address string `toml:"address"`
}

// String is used to return listener info
// tls (tcp 127.0.0.1:443)
func (l *Listener) String() string {
	return fmt.Sprintf("%s (%s %s)", l.Mode, l.Network, l.Address)
}

// Bootstrap is used to resolve bootstrap node listeners
type Bootstrap interface {
	// Validate is used to check bootstrap config correct
	Validate() error

	// Marshal is used to marshal bootstrap to []byte
	Marshal() ([]byte, error)

	// Unmarshal is used to unmarshal []byte to bootstrap
	Unmarshal([]byte) error

	// Resolve is used to resolve bootstrap node listeners
	Resolve() ([]*Listener, error)
}

// Load is used to create a bootstrap from config
func Load(
	ctx context.Context,
	mode string,
	config []byte,
	pool *proxy.Pool,
	client *dns.Client,
) (Bootstrap, error) {
	var bootstrap Bootstrap
	switch mode {
	case ModeHTTP:
		bootstrap = NewHTTP(ctx, pool, client)
	case ModeDNS:
		bootstrap = NewDNS(ctx, client)
	case ModeDirect:
		bootstrap = NewDirect()
	default:
		return nil, errors.Errorf("unknown mode: %s", mode)
	}
	err := bootstrap.Unmarshal(config)
	if err != nil {
		return nil, err
	}
	return bootstrap, nil
}
