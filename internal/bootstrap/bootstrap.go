package bootstrap

import (
	"project/internal/global/dnsclient"
	"project/internal/netx"
)

type Node struct {
	Mode    netx.Mode
	Network string
	Address string
}

type Bootstrap interface {
	Generate([]*Node) (string, error)
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	Resolve() ([]*Node, error)
}

type dns_resolver interface {
	Resolve(domain string, opts *dnsclient.Options) ([]string, error)
}
