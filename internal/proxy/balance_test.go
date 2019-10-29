package proxy

import (
	"net"
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
	// test testCompareClients (ha ha)
	ccc := make([]*Client, 4)
	for i := 0; i < 4; i++ {
		ccc[i] = new(Client)
	}
	testCompareClients(t, ccc)
}

func TestBalance(t *testing.T) {
	groups := testGenerateProxyGroup(t)
	balance, err := NewBalance("balance", groups.Clients()...)
	require.NoError(t, err)

	// test get next
	for i := 0; i < 4000; i++ {
		pcs := make([]*Client, 4)
		for j := 0; j < 4; j++ {
			pcs[j] = balance.getAndSetNext()
		}
		testCompareClients(t, pcs)
	}

	// test Connect
	timeout := balance.Timeout()
	network, address := balance.Server()
	conn, err := net.DialTimeout(network, address, timeout)
	require.NoError(t, err)
	pConn, err := balance.Connect(conn, "tcp", "8.8.8.8:53")
	require.NoError(t, err)
	_ = pConn.Close()

	testsuite.ProxyClient(t, &groups, balance)
}

func TestBalanceFailure(t *testing.T) {
	// no tag
	_, err := NewBalance("")
	require.Errorf(t, err, "empty balance tag")
	// no proxy clients
	_, err = NewBalance("chain-no-client")
	require.Errorf(t, err, "balance need at least one proxy client")

	groups := testGenerateProxyGroup(t)
	balance, err := NewBalance("balance-unreachable target", groups.Clients()...)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableTarget(t, &groups, balance)

	// connect unreachable target
	groupsC := testGenerateProxyGroup(t)
	defer func() { _ = groupsC.Close() }()
	balance, err = NewBalance("balance-connect unreachable target", groupsC.Clients()...)
	require.NoError(t, err)
	timeout := balance.Timeout()
	network, address := balance.Server()
	conn, err := net.DialTimeout(network, address, timeout)
	require.NoError(t, err)
	_, err = balance.Connect(conn, "tcp", "0.0.0.0:1")
	require.Error(t, err)
	testsuite.IsDestroyed(t, balance)
}
