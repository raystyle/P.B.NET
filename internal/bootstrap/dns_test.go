package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/xnet"
)

func TestDNS(t *testing.T) {
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
	DNS := NewDNS(nil)
	DNS.DomainName = "localhost"
	DNS.ListenerMode = xnet.TLS
	DNS.ListenerNetwork = "tcp"
	DNS.ListenerPort = "443"
	b, err := DNS.Marshal()
	require.NoError(t, err)
	DNS = NewDNS(new(mockDNSResolver))
	err = DNS.Unmarshal(b)
	require.NoError(t, err)
	resolved, err := DNS.Resolve()
	require.NoError(t, err)
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
	DNS = NewDNS(nil)
	DNS.DomainName = "localhost"
	DNS.ListenerMode = xnet.TLS
	DNS.ListenerNetwork = "tcp"
	DNS.ListenerPort = "443"
	DNS.Options.Type = dns.IPv6
	b, err = DNS.Marshal()
	require.NoError(t, err)
	DNS = NewDNS(new(mockDNSResolver))
	err = DNS.Unmarshal(b)
	require.NoError(t, err)
	resolved, err = DNS.Resolve()
	require.NoError(t, err)
	require.Equal(t, nodes, resolved)
}
