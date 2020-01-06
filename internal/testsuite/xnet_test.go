package testsuite

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListenerAndDial(t *testing.T) {
	if IPv4Enabled {
		const network = "tcp4"
		listener, err := net.Listen(network, "127.0.0.1:0")
		require.NoError(t, err)
		address := listener.Addr().String()
		ListenerAndDial(t, listener, func() (net.Conn, error) {
			return net.Dial(network, address)
		}, true)
	}

	if IPv6Enabled {
		const network = "tcp6"
		listener, err := net.Listen(network, "[::1]:0")
		require.NoError(t, err)
		address := listener.Addr().String()
		ListenerAndDial(t, listener, func() (net.Conn, error) {
			return net.Dial(network, address)
		}, true)
	}
}

func TestListenerAndDialContext(t *testing.T) {
	if IPv4Enabled {
		const network = "tcp4"
		listener, err := net.Listen(network, "127.0.0.1:0")
		require.NoError(t, err)
		address := listener.Addr().String()
		ListenerAndDial(t, listener, func() (net.Conn, error) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			return new(net.Dialer).DialContext(ctx, network, address)
		}, true)
	}

	if IPv6Enabled {
		const network = "tcp6"
		listener, err := net.Listen(network, "[::1]:0")
		require.NoError(t, err)
		address := listener.Addr().String()
		ListenerAndDial(t, listener, func() (net.Conn, error) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			return new(net.Dialer).DialContext(ctx, network, address)
		}, true)
	}
}

func TestConn(t *testing.T) {
	server, client := net.Pipe()
	ConnSC(t, server, client, true)
	server, client = net.Pipe()
	ConnCS(t, client, server, true)
}
