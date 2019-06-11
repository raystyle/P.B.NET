package bootstrap

import (
	"project/internal/connection"
	"project/internal/global/dnsclient"
)

type Node struct {
	Mode    connection.Mode
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
