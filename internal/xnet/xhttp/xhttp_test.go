package xhttp

import (
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestXHTTP(t *testing.T) {
	return
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
		// addr := listener.Addr().String()
		client, err := Dial(nil, nil, 0)
		require.NoError(t, err)
		wg.Wait()
		testsuite.Conn(t, server, client, true)
	}
	require.NoError(t, listener.Close())
	testsuite.IsDestroyed(t, listener)
}
