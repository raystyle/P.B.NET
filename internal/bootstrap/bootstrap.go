package bootstrap

import (
	"fmt"

	"project/internal/dns"
	"project/internal/proxy"
	"project/internal/xnet"
)

type Mode = string

const (
	ModeHTTP   Mode = "http"
	ModeDNS    Mode = "dns"
	ModeDirect Mode = "direct"
)

type Node struct {
	Mode    xnet.Mode `toml:"mode"`
	Network string    `toml:"network"`
	Address string    `toml:"address"`
}

type Bootstrap interface {
	Validate() error
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	Resolve() ([]*Node, error)
}

type DNSClient interface {
	Resolve(domain string, opts *dns.Options) ([]string, error)
}

type ProxyPool interface {
	Get(tag string) (*proxy.Client, error)
}

type fPanic struct {
	Mode Mode
	Err  error
}

func (f *fPanic) Error() string {
	return fmt.Sprintf("bootstrap %s internal error: %s", f.Mode, f.Err)
}

func Load(mode Mode, config []byte, pool ProxyPool, client DNSClient) (Bootstrap, error) {
	var bootstrap Bootstrap
	switch mode {
	case ModeHTTP:
		bootstrap = NewHTTP(pool, client)
	case ModeDNS:
		bootstrap = NewDNS(client)
	case ModeDirect:
		bootstrap = NewDirect(nil)
	default:
		return nil, fmt.Errorf("unknown bootstrap mode: %s", mode)
	}
	err := bootstrap.Unmarshal(config)
	if err != nil {
		return nil, err
	}
	return bootstrap, nil
}
