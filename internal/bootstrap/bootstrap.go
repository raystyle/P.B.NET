package bootstrap

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"project/internal/dns"
	"project/internal/proxy"
)

const (
	ModeHTTP   = "http"
	ModeDNS    = "dns"
	ModeDirect = "direct"
)

type Node struct {
	Mode    string `toml:"mode"`
	Network string `toml:"network"`
	Address string `toml:"address"`
}

// Bootstrap is used to get bootstrap nodes
type Bootstrap interface {
	// Validate is used to check bootstrap config correct
	Validate() error

	// Marshal is used to marshal bootstrap to []byte
	Marshal() ([]byte, error)

	// Unmarshal is used to unmarshal []byte to bootstrap
	Unmarshal([]byte) error

	// Resolve is used to get bootstrap nodes
	Resolve() ([]*Node, error)
}

// Load is used to make a bootstrap from config
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
		return nil, errors.Errorf("unknown bootstrap mode: %s", mode)
	}
	err := bootstrap.Unmarshal(config)
	if err != nil {
		return nil, err
	}
	return bootstrap, nil
}

type bPanic struct {
	Mode string
	Err  error
}

func (f *bPanic) String() string {
	return fmt.Sprintf("bootstrap %s internal error: %s", f.Mode, f.Err)
}
