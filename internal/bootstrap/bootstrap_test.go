package bootstrap

import (
	"errors"

	"project/internal/dns"
	"project/internal/global/dnsclient"
	"project/internal/netx"
)

func test_generate_bootstrap_nodes() []*Node {
	nodes := make([]*Node, 2)
	nodes[0] = &Node{
		Mode:    netx.TLS,
		Network: "tcp",
		Address: "127.0.0.1:53123",
	}
	nodes[1] = &Node{
		Mode:    netx.TLS,
		Network: "tcp",
		Address: "192.168.1.11:53123",
	}
	return nodes
}

type mock_resolver struct{}

func (this *mock_resolver) Resolve(domain string, opts *dnsclient.Options) ([]string, error) {
	if domain != "www.baidu.com" {
		return nil, errors.New("domain changed")
	}
	if opts == nil {
		opts = new(dnsclient.Options)
	}
	switch opts.Type {
	case "", dns.IPV4:
		return []string{"127.0.0.1", "192.168.1.11"}, nil
	case dns.IPV6:
		return []string{"[::1]", "[fe80::5456:5f8:1690:5792]"}, nil
	default:
		panic(dns.ERR_INVALID_TYPE)
	}
}
