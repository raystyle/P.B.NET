package bootstrap

import (
	"project/internal/connection"
)

func test_generate_bootstrap_node() []*Node {
	nodes := make([]*Node, 2)
	nodes[0] = &Node{
		Mode:    connection.TLS,
		Network: "tcp",
		Address: "127.0.0.1:53123",
	}
	nodes[1] = &Node{
		Mode:    connection.TLS,
		Network: "tcp",
		Address: "192.168.1.11:53123",
	}
	return nodes
}
