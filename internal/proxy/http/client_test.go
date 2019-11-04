package http

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestHTTPProxyClient(t *testing.T) {
	server := testGenerateHTTPServer(t)
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err := NewClient("tcp", server.Address(), &opts)
	require.NoError(t, err)
	testsuite.ProxyClient(t, server, client)
}

func TestHTTPSProxyClient(t *testing.T) {
	server, tlsConfig := testGenerateHTTPSServer(t)
	opts := Options{
		HTTPS:     true,
		Username:  "admin",
		TLSConfig: *tlsConfig,
	}
	client, err := NewClient("tcp", server.Address(), &opts)
	require.NoError(t, err)
	testsuite.ProxyClient(t, server, client)
}

func TestHTTPProxyClientWithoutPassword(t *testing.T) {
	server, err := NewServer("test", logger.Test, nil)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	client, err := NewClient("tcp", server.Address(), nil)
	require.NoError(t, err)
	testsuite.ProxyClient(t, server, client)
}

func TestHTTPProxyClientFailure(t *testing.T) {
	// unknown network
	_, err := NewClient("foo", "localhost:0", nil)
	require.Error(t, err)

	// connect unreachable proxy server
	client, err := NewClient("tcp", "localhost:0", nil)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableProxyServer(t, client)

	// connect unreachable target
	server := testGenerateHTTPServer(t)
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err = NewClient("tcp", server.Address(), &opts)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableTarget(t, server, client)
}

func TestHTTPSProxyClientFailure(t *testing.T) {
	// connect unreachable proxy server
	opts := Options{
		HTTPS: true,
	}
	client, err := NewClient("tcp", "localhost:0", &opts)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableProxyServer(t, client)

	// connect unreachable target
	server, tlsConfig := testGenerateHTTPSServer(t)
	opts = Options{
		HTTPS:     true,
		Username:  "admin",
		TLSConfig: *tlsConfig,
	}
	client, err = NewClient("tcp", server.Address(), &opts)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableTarget(t, server, client)
}
