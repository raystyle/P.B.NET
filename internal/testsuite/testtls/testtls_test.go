package testtls

import (
	"crypto/tls"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestOptionPair(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	serverCfg, clientCfg := OptionPair(t, "127.0.0.1")
	server, err := serverCfg.Apply()
	require.NoError(t, err)
	client, err := clientCfg.Apply()
	require.NoError(t, err)
	const network = "tcp"

	listener, err := tls.Listen(network, "localhost:0", server)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		return tls.Dial(network, address, client.Clone())
	}, true)
}
