package socks

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
	opts := Options{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, opts)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "admin", actual: opts.Username},
		{expected: "123456", actual: opts.Password},
		{expected: "test", actual: opts.UserID},
		{expected: time.Minute, actual: opts.Timeout},
		{expected: 1000, actual: opts.MaxConns},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

func TestSocks5ServerOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/socks5_server.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := Options{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "admin", actual: opts.Username},
		{expected: "123456", actual: opts.Password},
		{expected: time.Minute, actual: opts.Timeout},
		{expected: 1000, actual: opts.MaxConns},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

func TestSocks5ClientOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/socks5_client.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := Options{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "admin", actual: opts.Username},
		{expected: "123456", actual: opts.Password},
		{expected: time.Minute, actual: opts.Timeout},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

func TestSocks4aServerOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/socks4a_server.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := Options{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: testTag, actual: opts.UserID},
		{expected: time.Minute, actual: opts.Timeout},
		{expected: 1000, actual: opts.MaxConns},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

func TestSocks4aClientOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/socks4a_client.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := Options{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: testTag, actual: opts.UserID},
		{expected: time.Minute, actual: opts.Timeout},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

func TestSocks4ServerOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/socks4_server.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := Options{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: testTag, actual: opts.UserID},
		{expected: time.Minute, actual: opts.Timeout},
		{expected: 1000, actual: opts.MaxConns},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

func TestSocks4ClientOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/socks4_client.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := Options{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: testTag, actual: opts.UserID},
		{expected: time.Minute, actual: opts.Timeout},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

func TestCheckNetworkAndAddress(t *testing.T) {
	for _, network := range [...]string{
		"tcp", "tcp4", "tcp6",
		"udp", "udp4", "udp6",
	} {
		err := CheckNetworkAndAddress(network, "127.0.0.1:1")
		require.NoError(t, err)
	}
	err := CheckNetworkAndAddress("foo network", "127.0.0.1:1")
	require.EqualError(t, err, "unsupported network: foo network")

	err = CheckNetworkAndAddress("tcp", "127.0.0.1")
	require.EqualError(t, err, "missing port in address")
}
