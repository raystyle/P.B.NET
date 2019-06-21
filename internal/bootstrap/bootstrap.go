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
	Mode    xnet.Mode
	Network string
	Address string
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
	Mode Mode
	Err  error
}

func (this *fpanic) Error() string {
	return fmt.Sprintf("bootstrap %s internal error: %s", this.Mode, this.Err)
}
