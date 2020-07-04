package proxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/proxy/socks"
	"project/internal/random"
	"project/internal/testsuite"
)

func TestNewChain(t *testing.T) {
	t.Run("empty tag", func(t *testing.T) {
		_, err := NewChain("")
		require.Errorf(t, err, "empty proxy chain tag")
	})

	t.Run("no proxy clients", func(t *testing.T) {
		_, err := NewChain("chain-no-client")
		require.Errorf(t, err, "proxy chain need at least one proxy client")
	})
}

func TestChainSelectedClients(t *testing.T) {
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

	// padding
	_, _ = chain.Connect(context.Background(), nil, "", "")

	testsuite.ProxyClient(t, &groups, chain)
}

func TestChainWithHTTPSTarget(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	groups := testGenerateProxyGroup(t)
	// use select
	clients := make([]*Client, 4)
	clients[0] = groups["socks4a"].client
	clients[1] = groups["https"].client
	clients[2] = groups["http"].client
	clients[3] = groups["socks5"].client
	chain, err := NewChain("chain-no-socks4", clients...)
	require.NoError(t, err)

	testsuite.ProxyClientWithHTTPSTarget(t, chain)

	testsuite.IsDestroyed(t, chain)

	err = groups.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, &groups)
}

func TestChainWithRandomClients(t *testing.T) {
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
	fChainC := &Client{
		Tag:    fChain.tag,
		Mode:   ModeChain,
		client: fChain,
	}

	// make a balance that include chain
	clients := append(groups.Clients(), fChainC)
	fBalance, err := NewBalance("fBalance", clients...)
	require.NoError(t, err)
	fBalanceC := &Client{
		Tag:    fBalance.tag,
		Mode:   ModeBalance,
		client: fBalance,
	}

	// create final chain
	clients = append(groups.Clients(), fChainC, fBalanceC)
	chain, err := NewChain("chain-mix", clients...)
	require.NoError(t, err)

	testsuite.ProxyClient(t, &groups, chain)
}

func TestChain_connect(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	groups := testGenerateProxyGroup(t)

	client := groups["https"].client
	chain, err := NewChain("chain-target", client)
	require.NoError(t, err)

	testsuite.ProxyClientWithUnreachableTarget(t, &groups, chain)
}

func TestChainFailure(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	socks5Client, err := socks.NewSocks5Client("tcp", "localhost:0", nil)
	require.NoError(t, err)
	invalidClient := &Client{
		Mode:    ModeSocks5,
		Address: "127.0.0.1:0",
		client:  socks5Client,
	}

	t.Run("first proxy server failed", func(t *testing.T) {
		chain, err := NewChain("chain-connect", invalidClient)
		require.NoError(t, err)

		testsuite.ProxyClientWithUnreachableProxyServer(t, chain)
	})

	// the first connect successfully but second failed
	t.Run("second proxy server failed", func(t *testing.T) {
		groups := testGenerateProxyGroup(t)

		firstClient := groups["https"].client
		chain, err := NewChain("chain-unreachable", firstClient, invalidClient)
		require.NoError(t, err)

		testsuite.ProxyClientWithUnreachableTarget(t, &groups, chain)
	})
}
