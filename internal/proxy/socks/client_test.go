package socks

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestSocks5Client(t *testing.T) {
	t.Parallel()

	server := testGenerateSocks5Server(t)
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err := NewClient("tcp", server.Address(), &opts)
	require.NoError(t, err)
	testsuite.ProxyClient(t, server, client)
}

func TestSocks4aClient(t *testing.T) {
	t.Parallel()

	server := testGenerateSocks4aServer(t)
	opts := Options{
		Socks4: true,
		UserID: "admin",
	}
	client, err := NewClient("tcp", server.Address(), &opts)
	require.NoError(t, err)
	testsuite.ProxyClient(t, server, client)
}

func TestSocks5ClientWithoutPassword(t *testing.T) {
	t.Parallel()

	server, err := NewServer("test", logger.Test, nil)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	client, err := NewClient("tcp", server.Address(), nil)
	require.NoError(t, err)
	testsuite.ProxyClient(t, server, client)
}

func TestSocks4aClientWithoutUserID(t *testing.T) {
	t.Parallel()

	opts := &Options{Socks4: true}
	server, err := NewServer("test", logger.Test, opts)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	client, err := NewClient("tcp", server.Address(), opts)
	require.NoError(t, err)
	testsuite.ProxyClient(t, server, client)
}

func TestSocks5ClientFailure(t *testing.T) {
	t.Parallel()

	// unknown network
	_, err := NewClient("foo", "localhost:0", nil)
	require.Error(t, err)

	// connect unreachable proxy server
	client, err := NewClient("tcp", "localhost:0", nil)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableProxyServer(t, client)

	// connect unreachable target
	server := testGenerateSocks5Server(t)
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err = NewClient("tcp", server.Address(), &opts)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableTarget(t, server, client)
}

func TestSocks4aClientFailure(t *testing.T) {
	t.Parallel()

	// connect unreachable proxy server
	opts := Options{
		Socks4: true,
	}
	client, err := NewClient("tcp", "localhost:0", &opts)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableProxyServer(t, client)

	// connect unreachable target
	server := testGenerateSocks4aServer(t)
	opts = Options{
		Socks4: true,
		UserID: "admin",
	}
	client, err = NewClient("tcp", server.Address(), &opts)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableTarget(t, server, client)
}
