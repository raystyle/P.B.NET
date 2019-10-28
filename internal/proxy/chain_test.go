package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

func TestChain(t *testing.T) {
	groups := testGenerateGroup(t)
	defer func() {
		for _, group := range groups {
			group.Close(t)
		}
		testutil.IsDestroyed(t, &groups, 1)
	}()

	// use select
	clients := make([]*Client, 3)
	clients[0] = groups["http"].client
	// clients[1] = groups["https"].client
	clients[1] = groups["socks5"].client
	clients[2] = groups["socks4a"].client
	chain, err := NewChain("select", clients...)
	require.NoError(t, err)
	conn, err := chain.Dial("tcp", "github.com:443")
	require.NoError(t, err)
	_ = conn.Close()

	// use random
	getClients := func(n int) []*Client {
		if n > 4 {
			return nil
		}
		clients := make([]*Client, n)
		i := 0
		for _, group := range groups {
			if i < n {
				clients[i] = group.client
			} else {
				break
			}
			i += 1
		}
		return clients
	}
	chain, err = NewChain("two-chain", getClients(4)...)
	require.NoError(t, err)
	conn, err = chain.Dial("tcp", "github.com:443")
	require.NoError(t, err)
	_ = conn.Close()
}
