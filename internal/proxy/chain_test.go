package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/proxy/socks"
	"project/internal/random"
	"project/internal/testutil"
)

func TestProxyChainSelect(t *testing.T) {
	groups := testGenerateProxyGroup(t)
	// use select
	clients := make([]*Client, 4)
	clients[0] = groups["socks4a"].client
	clients[1] = groups["https"].client
	clients[2] = groups["http"].client
	clients[3] = groups["socks5"].client
	chain, err := NewChain("chain-select", clients...)
	require.NoError(t, err)
	testutil.ProxyClient(t, &groups, chain)
}

func TestProxyChainRandom(t *testing.T) {
	groups := testGenerateProxyGroup(t)
	clients := make([]*Client, 4)
	for ri := 0; ri < 3+random.Int(10); ri++ {
		i := 0
		for _, group := range groups {
			if i < 4 {
				clients[i] = group.client
			} else {
				break
			}
			i += 1
		}
	}
	chain, err := NewChain("chain-random", clients...)
	require.NoError(t, err)
	testutil.ProxyClient(t, &groups, chain)
}

func TestProxyChainFailure(t *testing.T) {
	socks5Client, err := socks.NewClient("tcp", "localhost:0", nil)
	require.NoError(t, err)
	chain, err := NewChain("chain-can't connect", &Client{
		Mode:   ModeSocks,
		client: socks5Client,
	})
	testutil.ProxyClientWithUnreachableProxyServer(t, chain)

	// the first connect successfully but second failed
}
