package testsuite

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListenerAndDial(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	if IPv4Enabled {
		testListenerAndDial(t, "tcp4")
	}
	if IPv6Enabled {
		testListenerAndDial(t, "tcp6")
	}
}

func testListenerAndDial(t *testing.T, network string) {
	listener, err := net.Listen(network, "localhost:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	ListenerAndDial(t, listener, func() (net.Conn, error) {
		return net.Dial(network, address)
	}, true)
}

func TestListenerAndDialContext(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	if IPv4Enabled {
		testListenerAndDialContext(t, "tcp4")
	}
	if IPv6Enabled {
		testListenerAndDialContext(t, "tcp6")
	}
}

func testListenerAndDialContext(t *testing.T, network string) {
	listener, err := net.Listen(network, "localhost:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	ListenerAndDial(t, listener, func() (net.Conn, error) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		return new(net.Dialer).DialContext(ctx, network, address)
	}, true)
}

func TestConn(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	server, client := net.Pipe()
	ConnSC(t, server, client, true)
	server, client = net.Pipe()
	ConnCS(t, client, server, true)
}

func TestPipeWithReaderWriter(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	PipeWithReaderWriter(t,
		func(conn net.Conn) {
			n, err := conn.Read(make([]byte, 4))
			require.NoError(t, err)
			require.Equal(t, 4, n)
		},
		func(conn net.Conn) {
			n, err := conn.Write(make([]byte, 4))
			require.NoError(t, err)
			require.Equal(t, 4, n)
		},
	)
}
