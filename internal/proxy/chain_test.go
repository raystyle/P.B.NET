package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/proxy/socks"
	"project/internal/random"
	"project/internal/testsuite"
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
	testsuite.ProxyClient(t, &groups, chain)
}

func TestProxyChainRandom(t *testing.T) {
	groups := testGenerateProxyGroup(t)
	chain, err := NewChain("chain-random", groups.Clients()...)
	require.NoError(t, err)
	testsuite.ProxyClient(t, &groups, chain)
}

func TestProxyChainWithSingleClient(t *testing.T) {
	groups := testGenerateProxyGroup(t)
	var client *Client
	for ri := 0; ri < 3+random.Int(10); ri++ {
		for _, group := range groups {
			client = group.client
		}
	}
	chain, err := NewChain("chain-random-single", client)
	require.NoError(t, err)
	testsuite.ProxyClient(t, &groups, chain)
}

func TestProxyChainFailure(t *testing.T) {
	// no tag
	_, err := NewChain("")
	require.Errorf(t, err, "empty proxy chain tag")
	// no proxy clients
	_, err = NewChain("chain-no-client")
	require.Errorf(t, err, "proxy chain need at least one proxy client")

	// unreachable first proxy server
	socks5Client, err := socks.NewClient("tcp", "localhost:0", nil)
	require.NoError(t, err)
	invalidClient := &Client{Mode: ModeSocks, client: socks5Client}
	chain, err := NewChain("chain-can't connect", invalidClient)
	testsuite.ProxyClientWithUnreachableProxyServer(t, chain)

	// the first connect successfully but second failed
	groups := testGenerateProxyGroup(t)
	firstClient := groups["https"].client
	chain, err = NewChain("chain-unreachable target", firstClient, invalidClient)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableTarget(t, &groups, chain)
}
