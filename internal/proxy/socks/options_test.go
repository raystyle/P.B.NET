package socks

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/toml"
)

func TestSocks5ServerOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/socks5_server.toml")
	require.NoError(t, err)
	opts := Options{}
	require.NoError(t, toml.Unmarshal(data, &opts))

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "admin", actual: opts.Username},
		{expected: "123456", actual: opts.Password},
		{expected: time.Minute, actual: opts.Timeout},
		{expected: 1000, actual: opts.MaxConns},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}

func TestSocks5ClientOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/socks5_client.toml")
	require.NoError(t, err)
	opts := Options{}
	require.NoError(t, toml.Unmarshal(data, &opts))

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "admin", actual: opts.Username},
		{expected: "123456", actual: opts.Password},
		{expected: time.Minute, actual: opts.Timeout},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}

func TestSocks4aServerOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/socks4a_server.toml")
	require.NoError(t, err)
	opts := Options{}
	require.NoError(t, toml.Unmarshal(data, &opts))

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "test", actual: opts.UserID},
		{expected: time.Minute, actual: opts.Timeout},
		{expected: 1000, actual: opts.MaxConns},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}

func TestSocks4aClientOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/socks4a_client.toml")
	require.NoError(t, err)
	opts := Options{}
	require.NoError(t, toml.Unmarshal(data, &opts))

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "test", actual: opts.UserID},
		{expected: time.Minute, actual: opts.Timeout},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}

func TestSocks4ServerOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/socks4_server.toml")
	require.NoError(t, err)
	opts := Options{}
	require.NoError(t, toml.Unmarshal(data, &opts))

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "test", actual: opts.UserID},
		{expected: time.Minute, actual: opts.Timeout},
		{expected: 1000, actual: opts.MaxConns},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}

func TestSocks4ClientOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/socks4_client.toml")
	require.NoError(t, err)
	opts := Options{}
	require.NoError(t, toml.Unmarshal(data, &opts))

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "test", actual: opts.UserID},
		{expected: time.Minute, actual: opts.Timeout},
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
