package light

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestLight(t *testing.T) {
	listener, err := Listen("tcp4", "localhost:0", 0)
	require.NoError(t, err)
	addr := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		return Dial("tcp4", addr, 0, nil)
	}, true)

	listener, err = Listen("tcp6", "localhost:0", 0)
	require.NoError(t, err)
	addr = listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		return Dial("tcp6", addr, 0, nil)
	}, true)
}

func TestLightConn(t *testing.T) {
	server, client := net.Pipe()
	server = Server(server, 0)
	client = Client(client, 0)
	testsuite.Conn(t, server, client, true)
}
