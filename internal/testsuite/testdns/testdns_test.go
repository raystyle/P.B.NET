package testdns

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/testsuite"
	"project/internal/testsuite/testproxy"
)

func TestDNSClient(t *testing.T) {
	client, _, manager := DNSClient(t)
	defer func() {
		require.NoError(t, manager.Close())
		testsuite.IsDestroyed(t, manager)
	}()
	const domain = "cloudflare-dns.com"
	if testsuite.EnableIPv4() {
		opts := &dns.Options{ServerTag: TagGoogleIPv4UDP}
		result, err := client.Resolve(domain, opts)
		require.NoError(t, err)
		t.Log("IPv4 UDP:", result)
		opts.ServerTag = TagGoogleIPv4DoT
		result, err = client.Resolve(domain, opts)
		require.NoError(t, err)
		t.Log("IPv4 DoH:", result)
		// use proxy
		opts.ProxyTag = testproxy.TagBalance
		result, err = client.Resolve(domain, opts)
		require.NoError(t, err)
		t.Log("IPv4 DoH(proxy):", result)

	}
	if testsuite.EnableIPv6() {
		opts := &dns.Options{ServerTag: TagMozillaIPv6UDP}
		result, err := client.Resolve(domain, opts)
		require.NoError(t, err)
		t.Log("IPv6 UDP:", result)
		opts.ServerTag = TagMozillaIPv6DoT
		result, err = client.Resolve(domain, opts)
		require.NoError(t, err)
		t.Log("IPv6 DoH:", result)
		// use proxy
		opts.ProxyTag = testproxy.TagBalance
		result, err = client.Resolve(domain, opts)
		require.NoError(t, err)
		t.Log("IPv6 DoH(proxy):", result)
	}

	testsuite.IsDestroyed(t, client)
}
