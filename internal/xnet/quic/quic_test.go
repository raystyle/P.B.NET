package quic

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

func TestQUIC(t *testing.T) {
	serverCfg, clientCfg := testutil.TLSConfigPair(t)
	listener, err := Listen("udp4", "localhost:0", serverCfg, 0)
	require.NoError(t, err)
	addr := listener.Addr().String()
	testutil.ListenerAndDial(t, listener, func() (net.Conn, error) {
		return Dial("udp4", addr, clientCfg, 0)
	}, true)

	listener, err = Listen("udp6", "localhost:0", serverCfg, 0)
	require.NoError(t, err)
	addr = listener.Addr().String()
	testutil.ListenerAndDial(t, listener, func() (net.Conn, error) {
		return Dial("udp6", addr, clientCfg, 0)
	}, true)
}
