package direct

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestDirect(t *testing.T) {
	testsuite.InitHTTPServers(t)

	d := Direct{}

	if testsuite.IPv4Enabled {
		const network = "tcp4"

		addr := "127.0.0.1:" + testsuite.HTTPServerPort
		conn, err := d.Dial(network, addr)
		require.NoError(t, err)
		testsuite.ProxyConn(t, conn)

		conn, err = d.DialContext(context.Background(), network, addr)
		require.NoError(t, err)
		testsuite.ProxyConn(t, conn)

		addr = "localhost:" + testsuite.HTTPServerPort
		conn, err = d.DialTimeout(network, addr, 0)
		require.NoError(t, err)
		testsuite.ProxyConn(t, conn)
	}

	if testsuite.IPv6Enabled {
		const network = "tcp6"

		addr := "[::1]:" + testsuite.HTTPServerPort
		conn, err := d.Dial(network, addr)
		require.NoError(t, err)
		testsuite.ProxyConn(t, conn)

		conn, err = d.DialContext(context.Background(), network, addr)
		require.NoError(t, err)
		testsuite.ProxyConn(t, conn)

		addr = "localhost:" + testsuite.HTTPServerPort
		conn, err = d.DialTimeout(network, addr, 0)
		require.NoError(t, err)
		testsuite.ProxyConn(t, conn)
	}

	// padding
	_, _ = d.Connect(nil, nil, "", "")
	d.HTTP(nil)
	t.Log(d.Timeout())
	t.Log(d.Server())
	t.Log(d.Info())
}
