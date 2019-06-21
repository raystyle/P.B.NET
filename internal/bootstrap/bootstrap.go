package bootstrap

import (
	"fmt"

	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/xnet"
)

type Mode string

const (
	M_HTTP   Mode = "http"
	M_DNS    Mode = "dns"
	M_DIRECT Mode = "direct"
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

type Config struct {
	Mode   Mode
	Config []byte
}

func Load(c *Config, p proxy_pool, d dns_resolver) (Bootstrap, error) {
	switch c.Mode {
	case M_HTTP:
		http := New_HTTP(p, d)
		err := http.Unmarshal(c.Config)
		if err != nil {
			return nil, err
		}
		return http, nil
	case M_DNS:
		dns := New_DNS(d)
		err := dns.Unmarshal(c.Config)
		if err != nil {
			return nil, err
		}
		return dns, nil
	case M_DIRECT:
		direct := New_Direct(nil)
		err := direct.Unmarshal(c.Config)
		if err != nil {
			return nil, err
		}
		return direct, nil
	default:
		return nil, fmt.Errorf("unknown bootstrap mode: %s", c.Mode)
	}
}

type dns_resolver interface {
	Resolve(domain string, opts *dnsclient.Options) ([]string, error)
}

type proxy_pool interface {
	Get(tag string) (*proxyclient.Client, error)
}

type fpanic struct {
	Mode Mode
	Err  error
}

func (this *fpanic) Error() string {
	return fmt.Sprintf("bootstrap %s internal error: %s", this.Mode, this.Err)
}
