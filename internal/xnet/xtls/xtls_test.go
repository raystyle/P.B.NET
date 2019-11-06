package xtls

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestXTLS(t *testing.T) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	if testsuite.EnableIPv4() {
		listener, err := Listen("tcp4", "localhost:0", serverCfg, 0)
		require.NoError(t, err)
		addr := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			return Dial("tcp4", addr, clientCfg, 0, nil)
		}, true)
	}

	if testsuite.EnableIPv6() {
		listener, err := Listen("tcp6", "localhost:0", serverCfg, 0)
		require.NoError(t, err)
		addr := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			return Dial("tcp6", addr, clientCfg, 0, nil)
		}, true)
	}
}

func TestXTLSConn(t *testing.T) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	server, client := net.Pipe()
	server = Server(server, serverCfg, 0)
	clientCfg.ServerName = "localhost"
	client = Client(client, clientCfg, 0)
	testsuite.Conn(t, server, client, false)
	testsuite.IsDestroyed(t, server)
	testsuite.IsDestroyed(t, client)
}
