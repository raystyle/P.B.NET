package direct

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestDirect(t *testing.T) {
	d := Direct{}

	if testsuite.EnableIPv4() {
		const network = "tcp4"

		addr := testsuite.GetIPv4Address()
		conn, err := d.Dial(network, addr)
		require.NoError(t, err)
		_ = conn.Close()

		addr = testsuite.GetIPv4Address()
		conn, err = d.DialContext(context.Background(), network, addr)
		require.NoError(t, err)
		_ = conn.Close()

		addr = testsuite.GetIPv4Address()
		conn, err = d.DialTimeout(network, addr, 0)
		require.NoError(t, err)
		_ = conn.Close()
	}

	if testsuite.EnableIPv6() {
		const network = "tcp6"

		addr := testsuite.GetIPv6Address()
		conn, err := d.Dial(network, addr)
		require.NoError(t, err)
		_ = conn.Close()

		addr = testsuite.GetIPv6Address()
		conn, err = d.DialContext(context.Background(), network, addr)
		require.NoError(t, err)
		_ = conn.Close()

		addr = testsuite.GetIPv6Address()
		conn, err = d.DialTimeout(network, addr, 0)
		require.NoError(t, err)
		_ = conn.Close()
	}

	// padding
	_, _ = d.Connect(nil, "", "")
	d.HTTP(nil)
	t.Log(d.Timeout())
	t.Log(d.Server())
	t.Log(d.Info())
}
