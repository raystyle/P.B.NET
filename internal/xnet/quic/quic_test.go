package quic

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestQUIC(t *testing.T) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	if testsuite.EnableIPv4() {
		listener, err := Listen("udp4", "localhost:0", serverCfg, 0)
		require.NoError(t, err)
		addr := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			return Dial("udp4", addr, clientCfg, 0)
		}, true)
	}

	if testsuite.EnableIPv6() {
		listener, err := Listen("udp6", "localhost:0", serverCfg, 0)
		require.NoError(t, err)
		addr := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			return Dial("udp6", addr, clientCfg, 0)
		}, true)
	}
}
