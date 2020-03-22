package lcx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOptions_Apply(t *testing.T) {
	opts := new(Options)
	opts = opts.apply()
	for _, testdata := range []struct {
		except interface{}
		actual interface{}
	}{
		{"tcp", opts.LocalNetwork},
		{":0", opts.LocalAddress},
		{DefaultConnectTimeout, opts.ConnectTimeout},
		{DefaultDialTimeout, opts.DialTimeout},
		{DefaultMaxConnections, opts.MaxConns},
	} {
		require.Equal(t, testdata.except, testdata.actual)
	}
}
