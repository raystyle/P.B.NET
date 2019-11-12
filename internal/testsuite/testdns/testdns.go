package testdns

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/proxy"
	"project/internal/testsuite/testproxy"
)

const (
	TagGoogleIPv4UDP     = "google_ipv4_udp"
	TagGoogleIPv4DoT     = "google_ipv4_dot"
	TagCloudflareIPv6UDP = "cloudflare_ipv6_udp"
	TagCloudflareIPv6DoT = "cloudflare_ipv6_dot"
)

// DNSClient is used to create a DNS client for test
func DNSClient(t *testing.T) (*dns.Client, *proxy.Pool, *proxy.Manager) {
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
	err = client.Add(TagCloudflareIPv6UDP, &dns.Server{
		Method:  dns.MethodUDP,
		Address: "[2606:4700:4700::64]:53",
	})
	require.NoError(t, err)
	err = client.Add(TagCloudflareIPv6DoT, &dns.Server{
		Method:  dns.MethodDoT,
		Address: "[2606:4700:4700::1111]:853",
	})
	require.NoError(t, err)
	return client, pool, manager
}
