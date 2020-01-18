package testsuite

import (
	"crypto/tls"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTLSConfigPair(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	serverCfg, clientCfg := TLSConfigPair(t)
	const network = "tcp"

	listener, err := tls.Listen(network, "localhost:0", serverCfg)
	require.NoError(t, err)
	address := listener.Addr().String()
	ListenerAndDial(t, listener, func() (net.Conn, error) {
		return tls.Dial(network, address, clientCfg.Clone())
	}, true)
}

func TestTLSConfigOptionPair(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	serverCfg, clientCfg := TLSConfigOptionPair(t)
	server, err := serverCfg.Apply()
	require.NoError(t, err)
	client, err := clientCfg.Apply()
	require.NoError(t, err)
	const network = "tcp"

	listener, err := tls.Listen(network, "localhost:0", server)
	require.NoError(t, err)
	address := listener.Addr().String()
	ListenerAndDial(t, listener, func() (net.Conn, error) {
		return tls.Dial(network, address, client.Clone())
	}, true)
}
