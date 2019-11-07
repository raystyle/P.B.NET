package bootstrap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/testsuite"
	"project/internal/testsuite/testdns"
	"project/internal/xnet"
)

func TestDNS(t *testing.T) {
	client, _, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()

	if testsuite.EnableIPv4() {
		nodes := []*Node{{
			Mode:    xnet.ModeTLS,
			Network: "tcp",
			Address: "127.0.0.1:443",
		}}
		DNS := NewDNS(nil, nil)
		DNS.DomainName = "localhost"
		DNS.ListenerMode = xnet.ModeTLS
		DNS.ListenerNetwork = "tcp"
		DNS.ListenerPort = "443"
		DNS.Options.Mode = dns.ModeSystem
		DNS.Options.Type = dns.TypeIPv4
		b, err := DNS.Marshal()
		require.NoError(t, err)
		DNS = NewDNS(context.Background(), client)
		err = DNS.Unmarshal(b)
		require.NoError(t, err)
		resolved, err := DNS.Resolve()
		require.NoError(t, err)
		require.Equal(t, nodes, resolved)
	}

	if testsuite.EnableIPv6() {
		nodes := []*Node{{
			Mode:    xnet.ModeTLS,
			Network: "tcp",
			Address: "[::1]:443",
		}}
		DNS := NewDNS(nil, nil)
		DNS.DomainName = "localhost"
		DNS.ListenerMode = xnet.ModeTLS
		DNS.ListenerNetwork = "tcp"
		DNS.ListenerPort = "443"
		DNS.Options.Mode = dns.ModeSystem
		DNS.Options.Type = dns.TypeIPv6
		b, err := DNS.Marshal()
		require.NoError(t, err)
		DNS = NewDNS(context.Background(), client)
		err = DNS.Unmarshal(b)
		require.NoError(t, err)
		resolved, err := DNS.Resolve()
		require.NoError(t, err)
		require.Equal(t, nodes, resolved)
	}
}
