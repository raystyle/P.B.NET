package testsuite

import (
	"crypto/tls"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTLSConfigPair(t *testing.T) {
	gm := MarkGoRoutines(t)
	defer gm.Compare()

	const network = "tcp"
	serverCfg, clientCfg := TLSConfigPair(t)
	listener, err := tls.Listen(network, "localhost:0", serverCfg)
	require.NoError(t, err)
	address := listener.Addr().String()
	ListenerAndDial(t, listener, func() (net.Conn, error) {
		return tls.Dial(network, address, clientCfg.Clone())
	}, true)
}

func TestTLSConfigOptionPair(t *testing.T) {
	gm := MarkGoRoutines(t)
	defer gm.Compare()

	serverCfg, clientCfg := TLSConfigOptionPair(t)
	sc, err := serverCfg.Apply()
	require.NoError(t, err)
	cc, err := clientCfg.Apply()
	require.NoError(t, err)

	const network = "tcp"
	listener, err := tls.Listen(network, "localhost:0", sc)
	require.NoError(t, err)
	address := listener.Addr().String()
	ListenerAndDial(t, listener, func() (net.Conn, error) {
		return tls.Dial(network, address, cc.Clone())
	}, true)
}
