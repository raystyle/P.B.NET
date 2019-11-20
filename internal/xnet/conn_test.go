package xnet

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
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
	t.Log(serverC)
	testsuite.Conn(t, serverC, clientC, true)
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

func TestConn_Receive_DataSize(t *testing.T) {
	server, client := net.Pipe()
	serverC := NewConn(server, time.Now())
	clientC := NewConn(client, time.Now())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, serverC.Close())
	}()
	_, err := clientC.Receive()
	require.Error(t, err)
	wg.Wait()
}

func TestConn_Receive_Data(t *testing.T) {
	server, client := net.Pipe()
	serverC := NewConn(server, time.Now())
	clientC := NewConn(client, time.Now())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := serverC.Write([]byte{0x00, 0x00, 0x10, 0x00})
		require.NoError(t, err)
		require.NoError(t, serverC.Close())
	}()
	_, err := clientC.Receive()
	require.Error(t, err)
	wg.Wait()
}
