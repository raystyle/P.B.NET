package xtls

import (
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

func TestXTLS(t *testing.T) {
	serverCfg, clientCfg := testutil.TLSConfigPair(t)
	listener, err := Listen("tcp", "localhost:0", serverCfg, 0)
	require.NoError(t, err)
	wg := sync.WaitGroup{}
	for i := 0; i < 3; i++ {
		var server net.Conn
		wg.Add(1)
		go func() {
			defer wg.Done()
			server, err = listener.Accept()
			require.NoError(t, err)
		}()
		addr := listener.Addr().String()
		client, err := Dial("tcp", addr, clientCfg, 0, nil)
		require.NoError(t, err)
		wg.Wait()
		testutil.Conn(t, server, client, true)
	}
	require.NoError(t, listener.Close())
	testutil.IsDestroyed(t, listener, 1)
}

func TestXTLSConn(t *testing.T) {
	serverCfg, clientCfg := testutil.TLSConfigPair(t)
	server, client := net.Pipe()
	server = Server(server, serverCfg, 0)
	clientCfg.ServerName = "localhost"
	client = Client(client, clientCfg, 0)
	testutil.Conn(t, server, client, false)
	testutil.IsDestroyed(t, server, 1)
	testutil.IsDestroyed(t, client, 1)
}
