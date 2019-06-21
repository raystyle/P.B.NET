package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/xnet"
)

const test_domain string = "localhost"

func Test_DNS(t *testing.T) {
	// ipv4
	nodes := make([]*Node, 2)
	nodes[0] = &Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "127.0.0.1:443",
	}
	nodes[1] = &Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "127.0.0.2:443",
	}
	DNS := New_DNS(nil)
	DNS.Domain = test_domain
	DNS.L_Mode = xnet.TLS
	DNS.L_Network = "tcp"
	DNS.L_Port = "443"
	b, err := DNS.Marshal()
	require.Nil(t, err, err)
	DNS = New_DNS(new(mock_resolver))
	err = DNS.Unmarshal(b)
	require.Nil(t, err, err)
	resolved, err := DNS.Resolve()
	require.Nil(t, err, err)
	require.Equal(t, nodes, resolved)
	// ipv6
	nodes = make([]*Node, 2)
	nodes[0] = &Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "[::1]:443",
	}
	nodes[1] = &Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "[::2]:443",
	}
	DNS = New_DNS(nil)
	DNS.Domain = test_domain
	DNS.L_Mode = xnet.TLS
	DNS.L_Network = "tcp"
	DNS.L_Port = "443"
	DNS.Options.Type = dns.IPV6
	b, err = DNS.Marshal()
	require.Nil(t, err, err)
	DNS = New_DNS(new(mock_resolver))
	err = DNS.Unmarshal(b)
	require.Nil(t, err, err)
	resolved, err = DNS.Resolve()
	require.Nil(t, err, err)
	require.Equal(t, nodes, resolved)
}
