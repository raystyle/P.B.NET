package light

import (
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

func TestLight(t *testing.T) {
	listener, err := Listen("tcp", "localhost:0", 0)
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
		client, err := Dial("tcp", addr, 0, nil)
		require.NoError(t, err)
		wg.Wait()
		testutil.Conn(t, server, client, true)
	}
	require.NoError(t, listener.Close())
	testutil.IsDestroyed(t, listener, 1)
}

func TestLightConn(t *testing.T) {
	server, client := net.Pipe()
	server = Server(server, 0)
	client = Client(client, 0)
	testutil.Conn(t, server, client, true)
}
