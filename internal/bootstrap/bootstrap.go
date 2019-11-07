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

type Bootstrap interface {
	Validate() error
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	Resolve() ([]*Node, error)
}

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
		bootstrap = NewDirect(nil)
	default:
		return nil, errors.Errorf("unknown bootstrap mode: %s", mode)
	}
	err := bootstrap.Unmarshal(config)
	if err != nil {
		return nil, err
	}
	return bootstrap, nil
}

type fPanic struct {
	Mode string
	Err  error
}

func (f *fPanic) Error() string {
	return fmt.Sprintf("bootstrap %s internal error: %s", f.Mode, f.Err)
}
