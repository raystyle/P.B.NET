package lcx

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/toml"
	"project/internal/testsuite"
)

func TestOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/options.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := new(Options)
	err = toml.Unmarshal(data, opts)
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, opts)

	opts = opts.apply()
	for _, testdata := range []struct {
		except interface{}
		actual interface{}
	}{
		{"tcp4", opts.LocalNetwork},
		{"127.0.0.1:1099", opts.LocalAddress},
		{15 * time.Second, opts.ConnectTimeout},
		{30 * time.Second, opts.DialTimeout},
		{100, opts.MaxConns},
	} {
		require.Equal(t, testdata.except, testdata.actual)
	}
}

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
