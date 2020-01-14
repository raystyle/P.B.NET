package proxy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

// example: a b c d
//
// 1. a compare b,c,d
// 2. b compare c,d
// 3. c compare d
func testCompareClients(t *testing.T, clients []*Client) {
	l := len(clients)
	for offset := 0; offset < l-1; offset++ {
		a := clients[offset]
		for i := offset + 1; i < l; i++ {
			if clients[i] == a {
				t.Fatal("same proxy client point")
			}
		}
	}
}

func TestCompareClients(t *testing.T) {
	ccc := make([]*Client, 4)
	for i := 0; i < 4; i++ {
		ccc[i] = new(Client)
	}
	testCompareClients(t, ccc)
}

func TestBalance_GetAndSelectNext(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	groups := testGenerateProxyGroup(t)
	defer func() { require.NoError(t, groups.Close()) }()

	balance, err := NewBalance("balance", groups.Clients()...)
	require.NoError(t, err)

	for i := 0; i < 4000; i++ {
		pcs := make([]*Client, 4)
		for j := 0; j < 4; j++ {
			pcs[j] = balance.GetAndSelectNext()
		}
		testCompareClients(t, pcs)
	}
}

func TestBalance(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	groups := testGenerateProxyGroup(t)
	balance, err := NewBalance("balance", groups.Clients()...)
	require.NoError(t, err)

	_, _ = balance.Connect(nil, nil, "", "")

	testsuite.ProxyClient(t, &groups, balance)
}

func TestBalanceFailure(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	// empty tag
	_, err := NewBalance("")
	require.Errorf(t, err, "empty balance tag")
	// no proxy clients
	_, err = NewBalance("chain-no-client")
	require.Errorf(t, err, "balance need at least one proxy client")

	groups := testGenerateProxyGroup(t)
	balance, err := NewBalance("01", groups.Clients()...)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableTarget(t, &groups, balance)
}

func testGenerateBalanceInBalance(t *testing.T) (groups, *Balance) {
	groups := testGenerateProxyGroup(t)
	clients := make([]*Client, 3)
	b1, err := NewBalance("balance-1", groups.Clients()...)
	require.NoError(t, err)
	clients[0] = &Client{Tag: b1.tag, Mode: ModeBalance, client: b1}
	b2, err := NewBalance("balance-2", groups.Clients()...)
	require.NoError(t, err)
	clients[1] = &Client{Tag: b2.tag, Mode: ModeBalance, client: b2}
	b3, err := NewBalance("balance-3", groups.Clients()...)
	require.NoError(t, err)
	clients[2] = &Client{Tag: b3.tag, Mode: ModeBalance, client: b3}
	fb, err := NewBalance("final-balance", clients...)
	require.NoError(t, err)
	return groups, fb
}

func TestBalanceInBalance(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	groups, fb := testGenerateBalanceInBalance(t)
	testsuite.ProxyClient(t, &groups, fb)
}

func TestPrintInfo(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	groups, fb := testGenerateBalanceInBalance(t)
	defer func() { _ = groups.Close() }()
	fmt.Println(fb.Info())

	// create a chain
	c1 := groups.Clients()[0]
	c2 := &Client{Tag: fb.tag, Mode: ModeBalance, client: fb}
	c3 := groups.Clients()[1]
	chain, err := NewChain("chain-mix", c1, c2, c3)
	require.NoError(t, err)
	fmt.Println(chain.Info())

	// create a balance with chain
	cc := &Client{Tag: chain.tag, Mode: ModeChain, client: chain}
	balance, err := NewBalance("balance-mix", c1, cc, c3)
	require.NoError(t, err)
	fmt.Println(balance.Info())
}
