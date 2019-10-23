package xtls

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

func TestXTLS(t *testing.T) {
	serverCfg, clientCfg := testutil.TLSConfigPair(t)
	listener, err := Listen("tcp4", "localhost:0", serverCfg, 0)
	require.NoError(t, err)
	addr := listener.Addr().String()
	testutil.ListenerAndDial(t, listener, func() (net.Conn, error) {
		return Dial("tcp4", addr, clientCfg, 0, nil)
	}, true)

	listener, err = Listen("tcp6", "localhost:0", serverCfg, 0)
	require.NoError(t, err)
	addr = listener.Addr().String()
	testutil.ListenerAndDial(t, listener, func() (net.Conn, error) {
		return Dial("tcp6", addr, clientCfg, 0, nil)
	}, true)
}

func TestXTLSConn(t *testing.T) {
	serverCfg, clientCfg := testutil.TLSConfigPair(t)
	server, client := net.Pipe()
	server = Server(server, serverCfg, 0)
	clientCfg.ServerName = "localhost"
	client = Client(client, clientCfg, 0)
	testutil.Conn(t, server, client, false)
	testutil.IsDestroyed(t, server, 1)
	testutil.IsDestroyed(t, client, 1)
}
