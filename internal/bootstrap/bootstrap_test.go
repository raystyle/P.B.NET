package bootstrap

import (
	"project/internal/xnet"
)

func testGenerateNodes() []*Node {
	nodes := make([]*Node, 2)
	nodes[0] = &Node{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "127.0.0.1:53123",
	}
	nodes[1] = &Node{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "[::1]:53123",
	}
	return nodes
}
