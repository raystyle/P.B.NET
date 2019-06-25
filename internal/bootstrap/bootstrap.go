package bootstrap

import (
	"fmt"

	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/xnet"
)

type Mode = string

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

type dns_resolver interface {
	Resolve(domain string, opts *dnsclient.Options) ([]string, error)
}

type proxy_pool interface {
	Get(tag string) (*proxyclient.Client, error)
}

type fpanic struct {
	M Mode
	E error
}

func (this *fpanic) Error() string {
	return fmt.Sprintf("bootstrap %s internal error: %s", this.M, this.E)
}

func Load(m Mode, config []byte, p proxy_pool, d dns_resolver) (Bootstrap, error) {
	switch m {
	case M_HTTP:
		http := New_HTTP(p, d)
		err := http.Unmarshal(config)
		if err != nil {
			return nil, err
		}
		return http, nil
	case M_DNS:
		dns := New_DNS(d)
		err := dns.Unmarshal(config)
		if err != nil {
			return nil, err
		}
		return dns, nil
	case M_DIRECT:
		direct := New_Direct(nil)
		err := direct.Unmarshal(config)
		if err != nil {
			return nil, err
		}
		return direct, nil
	default:
		return nil, fmt.Errorf("unknown bootstrap mode: %s", m)
	}
}
