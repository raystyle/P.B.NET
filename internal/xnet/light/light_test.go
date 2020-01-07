package light

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

// prevent block in Dial()
type tListener struct {
	net.Listener
}

func (l *tListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	err = conn.(*Conn).Handshake()
	if err != nil {
		return nil, err
	}
	return conn, err
}

func TestListenAndDial(t *testing.T) {
	if testsuite.IPv4Enabled {
		const network = "tcp4"
		listener, err := Listen(network, "127.0.0.1:0", 0)
		require.NoError(t, err)
		listener = &tListener{Listener: listener}
		address := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			return Dial(network, address, 0, nil)
		}, true)
	}

	if testsuite.IPv6Enabled {
		const network = "tcp6"
		listener, err := Listen(network, "[::1]:0", 0)
		require.NoError(t, err)
		listener = &tListener{Listener: listener}
		address := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			return Dial(network, address, 0, nil)
		}, true)
	}
}

func TestListenAndDialContext(t *testing.T) {
	if testsuite.IPv4Enabled {
		const network = "tcp4"
		listener, err := Listen(network, "127.0.0.1:0", 0)
		require.NoError(t, err)
		listener = &tListener{Listener: listener}
		address := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			return DialContext(ctx, network, address, 0, nil)
		}, true)
	}

	if testsuite.IPv6Enabled {
		const network = "tcp6"
		listener, err := Listen(network, "[::1]:0", 0)
		require.NoError(t, err)
		listener = &tListener{Listener: listener}
		address := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			return DialContext(ctx, network, address, 0, nil)
		}, true)
	}
}
