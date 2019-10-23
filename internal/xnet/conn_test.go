package xnet

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

func TestConn(t *testing.T) {
	server, client := net.Pipe()
	serverC := NewConn(server, time.Now())
	clientC := NewConn(client, time.Now())
	msg := []byte("hello server")
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		received, err := serverC.Receive()
		require.NoError(t, err)
		require.Equal(t, msg, received)
	}()
	require.NoError(t, clientC.Send(msg))
	wg.Wait()
	t.Log(serverC.Status())
	testutil.Conn(t, serverC, clientC, true)
}

func TestConnWithTooBigMessage(t *testing.T) {
	server, client := net.Pipe()
	serverC := NewConn(server, time.Now())
	clientC := NewConn(client, time.Now())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := serverC.Receive()
		require.Error(t, err)
	}()
	_, err := clientC.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	require.NoError(t, err)
	wg.Wait()
}
