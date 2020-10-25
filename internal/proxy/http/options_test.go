package http

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
		{expected: time.Minute, actual: opts.Timeout},
		{expected: "keep-alive", actual: opts.Header.Get("Connection")},
		{expected: 1000, actual: opts.MaxConns},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

func TestHTTPServerOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/http_server.toml")
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
		{expected: 10 * time.Second, actual: opts.Server.ReadTimeout},
		{expected: 2, actual: opts.Transport.MaxIdleConns},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

func TestHTTPClientOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/http_client.toml")
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
		{expected: "keep-alive", actual: opts.Header.Get("Connection")},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

func TestHTTPSServerOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/https_server.toml")
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
		{expected: 1, actual: len(opts.Server.TLSConfig.Certificates)},
		{expected: 2, actual: opts.Transport.MaxIdleConns},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

func TestHTTPSClientOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/https_client.toml")
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
		{expected: "keep-alive", actual: opts.Header.Get("Connection")},
		{expected: 1, actual: len(opts.TLSConfig.RootCAs)},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

func TestCheckNetwork(t *testing.T) {
	for _, network := range [...]string{
		"tcp", "tcp4", "tcp6",
	} {
		err := CheckNetwork(network)
		require.NoError(t, err)
	}
	err := CheckNetwork("foo network")
	require.EqualError(t, err, "unsupported network: foo network")
}
