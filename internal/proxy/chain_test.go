package proxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/proxy/socks"
	"project/internal/random"
	"project/internal/testsuite"
)

func TestChainSelect(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	groups := testGenerateProxyGroup(t)
	// use select
	clients := make([]*Client, 5)
	clients[0] = groups["socks4a"].client
	clients[1] = groups["https"].client
	clients[2] = groups["http"].client
	clients[3] = groups["socks5"].client
	clients[4] = groups["socks4"].client
	chain, err := NewChain("chain-select", clients...)
	require.NoError(t, err)

	_, _ = chain.Connect(context.Background(), nil, "", "")

	testsuite.ProxyClient(t, &groups, chain)
}

func TestChainRandom(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	groups := testGenerateProxyGroup(t)

	chain, err := NewChain("chain-random", groups.Clients()...)
	require.NoError(t, err)

	testsuite.ProxyClient(t, &groups, chain)
}

func TestChainWithSingleClient(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	groups := testGenerateProxyGroup(t)
	var client *Client
	for ri := 0; ri < 3+random.Int(10); ri++ {
		for _, group := range groups {
			client = group.client
		}
	}

	chain, err := NewChain("chain-single", client)
	require.NoError(t, err)

	testsuite.ProxyClient(t, &groups, chain)
}

func TestChainWithMixClient(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	groups := testGenerateProxyGroup(t)
	// make a chain
	fChain, err := NewChain("fChain", groups.Clients()...)
	require.NoError(t, err)
	fChainC := &Client{Tag: fChain.tag, Mode: ModeChain, client: fChain}
	// make a balance that include chain
	fBalance, err := NewBalance("fBalance", append(groups.Clients(), fChainC)...)
	require.NoError(t, err)
	fBalanceC := &Client{Tag: fBalance.tag, Mode: ModeBalance, client: fBalance}
	// create final chain
	chain, err := NewChain("chain-mix", append(groups.Clients(), fChainC, fBalanceC)...)
	require.NoError(t, err)

	testsuite.ProxyClient(t, &groups, chain)
}

func TestChainFailure(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	// no tag
	_, err := NewChain("")
	require.Errorf(t, err, "empty proxy chain tag")
	// no proxy clients
	_, err = NewChain("chain-no-client")
	require.Errorf(t, err, "proxy chain need at least one proxy client")

	// unreachable first proxy server
	socks5Client, err := socks.NewSocks5Client("tcp", "localhost:0", nil)
	require.NoError(t, err)
	invalidClient := &Client{Mode: ModeSocks5, client: socks5Client}
	chain, err := NewChain("chain-can't connect", invalidClient)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableProxyServer(t, chain)

	// the first connect successfully but second failed
	groups := testGenerateProxyGroup(t)
	firstClient := groups["https"].client
	chain, err = NewChain("chain-unreachable target", firstClient, invalidClient)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableTarget(t, &groups, chain)
}
