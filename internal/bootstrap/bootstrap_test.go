package bootstrap

import (
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
