package direct

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirect(t *testing.T) {
	d := Direct{}
	conn, err := d.Dial("tcp", "github.com:443")
	require.NoError(t, err)
	_ = conn.Close()
	conn, err = d.DialContext(context.Background(), "tcp", "github.com:443")
	require.NoError(t, err)
	_ = conn.Close()
	conn, err = d.DialTimeout("tcp", "github.com:443", 0)
	require.NoError(t, err)
	_ = conn.Close()
	d.HTTP(nil)
	_, _ = d.Connect(nil, "", "")
	t.Log(d.Timeout())
	t.Log(d.Server())
	t.Log(d.Info())
}
