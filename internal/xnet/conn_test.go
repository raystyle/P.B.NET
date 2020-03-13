package xnet

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestConn(t *testing.T) {
	server, client := net.Pipe()
	serverC := NewConn(server, ModePipe, time.Now())
	clientC := NewConn(client, ModePipe, time.Now())
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
	t.Logf("mode:%s\n", serverC.Mode())
	t.Log("status:\n", serverC.Status())
	t.Logf("string:\n%s", serverC)
	testsuite.ConnSC(t, serverC, clientC, true)
}

func TestConnWithTooBigMessage(t *testing.T) {
	server, client := net.Pipe()
	serverC := NewConn(server, ModePipe, time.Now())
	clientC := NewConn(client, ModePipe, time.Now())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := serverC.Receive()
		require.Equal(t, ErrReceiveTooBigMessage, err)
	}()
	err := clientC.Send(bytes.Repeat([]byte{0}, 256<<10+1))
	require.Equal(t, ErrSendTooBigMessage, err)
	_, err = clientC.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	require.NoError(t, err)
	wg.Wait()
}

func TestConnReceiveHeader(t *testing.T) {
	server, client := net.Pipe()
	serverC := NewConn(server, ModePipe, time.Now())
	clientC := NewConn(client, ModePipe, time.Now())
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

func TestConnClosed(t *testing.T) {
	server, client := net.Pipe()
	serverC := NewConn(server, ModePipe, time.Now())
	clientC := NewConn(client, ModePipe, time.Now())
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
