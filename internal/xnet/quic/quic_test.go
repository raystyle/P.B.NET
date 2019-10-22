package quic

import (
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

func TestQUIC(t *testing.T) {
	testutil.PPROF()
	serverCfg, clientCfg := testutil.TLSConfigPair(t)
	listener, err := Listen("udp", "localhost:0", serverCfg, 0)
	require.NoError(t, err)
	go func() {
		// time.Sleep(3 * time.Second)
		// listener.Close()
	}()

	wg := sync.WaitGroup{}
	for i := 0; i < 3; i++ {
		var server net.Conn
		wg.Add(1)
		go func() {
			defer wg.Done()
			server, err = listener.Accept()

			fmt.Println("asdsdasdasd")

			fmt.Println("asdsdsss", server, err)
			require.NoError(t, err)

			fmt.Println("asdsd")
		}()
		client, err := Dial("udp", listener.Addr().String(), clientCfg, 0)
		require.NoError(t, err)
		wg.Wait()
		testutil.Conn(t, server, client, true)
	}
	require.NoError(t, listener.Close())
	testutil.IsDestroyed(t, listener, 1)
}
