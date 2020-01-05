package testsuite

import (
	"crypto/tls"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTLSConfigPair(t *testing.T) {
	serverCfg, clientCfg := TLSConfigPair(t)
	listener, err := tls.Listen("tcp", "localhost:0", serverCfg)
	require.NoError(t, err)
	var server net.Conn
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		server, err = listener.Accept()
		require.NoError(t, err)
		// must Handshake, otherwise tls.Dial() will block
		require.NoError(t, server.(*tls.Conn).Handshake())
	}()
	client, err := tls.Dial("tcp", listener.Addr().String(), clientCfg)
	require.NoError(t, err)
	wg.Wait()
	ConnSC(t, server, client, true)
}

func TestTLSConfigOptionPair(t *testing.T) {
	serverCfg, clientCfg := TLSConfigOptionPair(t)
	sc, err := serverCfg.Apply()
	require.NoError(t, err)
	listener, err := tls.Listen("tcp", "localhost:0", sc)
	require.NoError(t, err)
	var server net.Conn
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		server, err = listener.Accept()
		require.NoError(t, err)
		// must Handshake, otherwise tls.Dial() will block
		require.NoError(t, server.(*tls.Conn).Handshake())
	}()
	cc, err := clientCfg.Apply()
	require.NoError(t, err)
	client, err := tls.Dial("tcp", listener.Addr().String(), cc)
	require.NoError(t, err)
	wg.Wait()
	ConnCS(t, client, server, true)
}
