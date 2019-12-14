package proxy

import (
	"context"
	"net"
	"strings"
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
	t.Parallel()

	groups := testGenerateProxyGroup(t)
	balance, err := NewBalance("balance", groups.Clients()...)
	require.NoError(t, err)

	// test get next
	for i := 0; i < 4000; i++ {
		pcs := make([]*Client, 4)
		for j := 0; j < 4; j++ {
			pcs[j] = balance.getAndSelect()
		}
		testCompareClients(t, pcs)
	}

	// test Connect
	testsuite.InitHTTPServers(t)
	if testsuite.EnableIPv4() {
		timeout := balance.Timeout()
		network, address := balance.Server()
		conn, err := net.DialTimeout(network, address, timeout)
		require.NoError(t, err)
		addr := "127.0.0.1:" + testsuite.HTTPServerPort
		pConn, err := balance.Connect(context.Background(), conn, "tcp4", addr)
		require.NoError(t, err)
		testsuite.ProxyConn(t, pConn)
	}
	if testsuite.EnableIPv6() {
		// remove socks4
		var clients []*Client
		for _, client := range groups.Clients() {
			if !strings.Contains(client.Info(), "socks4") {
				clients = append(clients, client)
			}
		}
		balance, err := NewBalance("balance", clients...)
		require.NoError(t, err)
		// test
		timeout := balance.Timeout()
		network, address := balance.Server()
		conn, err := net.DialTimeout(network, address, timeout)
		require.NoError(t, err)
		addr := "[::1]:" + testsuite.HTTPServerPort
		pConn, err := balance.Connect(context.Background(), conn, "tcp6", addr)
		require.NoError(t, err)
		testsuite.ProxyConn(t, pConn)
	}

	testsuite.ProxyClient(t, &groups, balance)
}

func TestBalanceFailure(t *testing.T) {
	t.Parallel()

	// no tag
	_, err := NewBalance("")
	require.Errorf(t, err, "empty balance tag")
	// no proxy clients
	_, err = NewBalance("chain-no-client")
	require.Errorf(t, err, "balance need at least one proxy client")

	groups := testGenerateProxyGroup(t)
	balance, err := NewBalance("01", groups.Clients()...)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableTarget(t, &groups, balance)

	// connect unreachable target
	groupsC := testGenerateProxyGroup(t)
	defer func() { _ = groupsC.Close() }()
	balance, err = NewBalance("02", groupsC.Clients()...)
	require.NoError(t, err)
	timeout := balance.Timeout()
	network, address := balance.Server()
	conn, err := net.DialTimeout(network, address, timeout)
	require.NoError(t, err)
	_, err = balance.Connect(context.Background(), conn, "tcp", "0.0.0.0:1")
	require.Error(t, err)
	testsuite.IsDestroyed(t, balance)
}
