package testsuite

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListenerAndDial(t *testing.T) {
	if EnableIPv4() {
		l, err := net.Listen("tcp4", "localhost:0")
		require.NoError(t, err)
		addr := l.Addr().String()
		ListenerAndDial(t, l, func() (net.Conn, error) {
			return net.Dial("tcp4", addr)
		}, true)
	}

	if EnableIPv6() {
		l, err := net.Listen("tcp6", "localhost:0")
		require.NoError(t, err)
		addr := l.Addr().String()
		ListenerAndDial(t, l, func() (net.Conn, error) {
			return net.Dial("tcp6", addr)
		}, true)
	}
}

func TestConn(t *testing.T) {
	server, client := net.Pipe()
	Conn(t, server, client, true)
}
