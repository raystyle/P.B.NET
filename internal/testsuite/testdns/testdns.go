package testdns

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/proxy"
	"project/internal/testsuite/testproxy"
)

const (
	TagGoogleIPv4UDP  = "google_ipv4_udp"
	TagGoogleIPv4DoT  = "google_ipv4_dot"
	TagMozillaIPv6UDP = "mozilla_ipv6_udp"
	TagMozillaIPv6DoT = "mozilla_ipv6_dot"
)

// DNSClient is used to create a DNS client for test
func DNSClient(t *testing.T) (*dns.Client, *proxy.Manager) {
	pool, manager := testproxy.PoolAndManager(t)
	client := dns.NewClient(pool)
	err := client.Add(TagGoogleIPv4UDP, &dns.Server{
		Method:  dns.MethodUDP,
		Address: "8.8.8.8:53",
	})
	require.NoError(t, err)
	err = client.Add(TagGoogleIPv4DoT, &dns.Server{
		Method:  dns.MethodDoT,
		Address: "8.8.8.8:853",
	})
	require.NoError(t, err)
	err = client.Add(TagMozillaIPv6UDP, &dns.Server{
		Method:  dns.MethodUDP,
		Address: "[2606:4700::6810:f8f9]:53",
	})
	require.NoError(t, err)
	err = client.Add(TagMozillaIPv6DoT, &dns.Server{
		Method:  dns.MethodDoT,
		Address: "[2606:4700::6810:f8f9]:853",
	})
	require.NoError(t, err)
	return client, manager
}
