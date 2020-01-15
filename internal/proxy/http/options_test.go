package http

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"
)

func TestServerOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/server.toml")
	require.NoError(t, err)
	opts := Options{}
	require.NoError(t, toml.Unmarshal(data, &opts))

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: true, actual: opts.HTTPS},
		{expected: "admin", actual: opts.Username},
		{expected: "123456", actual: opts.Password},
		{expected: time.Minute, actual: opts.Timeout},
		{expected: 1000, actual: opts.MaxConns},
		{expected: 10 * time.Second, actual: opts.Server.ReadTimeout},
		{expected: 2, actual: opts.Transport.MaxIdleConns},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}

func TestClientOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/client.toml")
	require.NoError(t, err)
	opts := Options{}
	require.NoError(t, toml.Unmarshal(data, &opts))

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: true, actual: opts.HTTPS},
		{expected: "admin", actual: opts.Username},
		{expected: "123456", actual: opts.Password},
		{expected: time.Minute, actual: opts.Timeout},
		{expected: "test.com", actual: opts.TLSConfig.ServerName},
		{expected: "keep-alive", actual: opts.Header.Get("Connection")},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}

func TestCheckNetwork(t *testing.T) {
	for _, network := range []string{"tcp", "tcp4", "tcp6"} {
		require.NoError(t, CheckNetwork(network))
	}
	err := CheckNetwork("foo network")
	require.Error(t, err)
	t.Log(err)
}
