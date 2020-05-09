package direct

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestDirect(t *testing.T) {
	testsuite.InitHTTPServers(t)

	direct := Direct{}
	background := context.Background()

	if testsuite.IPv4Enabled {
		t.Run("IPv4", func(t *testing.T) {
			const network = "tcp4"

			addr := "127.0.0.1:" + testsuite.HTTPServerPort
			conn, err := direct.Dial(network, addr)
			require.NoError(t, err)
			testsuite.ProxyConn(t, conn)

			conn, err = direct.DialContext(background, network, addr)
			require.NoError(t, err)
			testsuite.ProxyConn(t, conn)

			addr = "localhost:" + testsuite.HTTPServerPort
			conn, err = direct.DialTimeout(network, addr, 0)
			require.NoError(t, err)
			testsuite.ProxyConn(t, conn)
		})
	}

	if testsuite.IPv6Enabled {
		t.Run("IPv6", func(t *testing.T) {
			const network = "tcp6"

			addr := "[::1]:" + testsuite.HTTPServerPort
			conn, err := direct.Dial(network, addr)
			require.NoError(t, err)
			testsuite.ProxyConn(t, conn)

			conn, err = direct.DialContext(background, network, addr)
			require.NoError(t, err)
			testsuite.ProxyConn(t, conn)

			addr = "localhost:" + testsuite.HTTPServerPort
			conn, err = direct.DialTimeout(network, addr, 0)
			require.NoError(t, err)
			testsuite.ProxyConn(t, conn)
		})
	}

	// padding
	_, _ = direct.Connect(background, nil, "", "")
	direct.HTTP(nil)
	t.Log(direct.Timeout())
	t.Log(direct.Server())
	t.Log(direct.Info())
}
