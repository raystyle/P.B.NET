package socks

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestSocks5Client(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks5Server(t)
	address := server.Addresses()[0].String()
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err := NewClient("tcp", address, &opts)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestSocks4aClient(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks4aServer(t)
	address := server.Addresses()[0].String()
	opts := Options{
		UserID: "admin",
	}
	client, err := NewClient("tcp", address, &opts)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestSocks4Client(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks4Server(t)
	address := server.Addresses()[0].String()
	opts := Options{
		UserID: "admin",
	}
	client, err := NewClient("tcp", address, &opts)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestSocks5ClientCancelConnect(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks5Server(t)
	address := server.Addresses()[0].String()
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err := NewClient("tcp", address, &opts)
	require.NoError(t, err)

	testsuite.ProxyClientCancelConnect(t, server, client)
}

func TestSocks5ClientWithoutPassword(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewSocks5Server("test", logger.Test, nil)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe("tcp", "localhost:0")
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	address := server.Addresses()[0].String()
	client, err := NewClient("tcp", address, nil)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestSocks4aClientWithoutUserID(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewSocks4aServer("test", logger.Test, nil)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe("tcp", "localhost:0")
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	address := server.Addresses()[0].String()
	client, err := NewClient("tcp", address, nil)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestSocks5Authenticate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks5Server(t)
	defer func() {
		require.NoError(t, server.Close())
		testsuite.IsDestroyed(t, server)
	}()
	address := server.Addresses()[0].String()

	opt := Options{
		Username: "admin",
		Password: "123457",
	}
	client, err := NewClient("tcp", address, &opt)
	require.NoError(t, err)
	_, err = client.Dial("tcp", "localhost:0")
	require.Error(t, err)
}

func TestSocks4aUserID(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks4aServer(t)
	defer func() {
		require.NoError(t, server.Close())
		testsuite.IsDestroyed(t, server)
	}()
	address := server.Addresses()[0].String()

	opt := Options{
		UserID: "foo-user-id",
	}
	client, err := NewClient("tcp", address, &opt)
	require.NoError(t, err)
	_, err = client.Dial("tcp", "localhost:0")
	require.Error(t, err)
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
	address := server.Addresses()[0].String()

	client, err = NewClient("tcp", address, &opts)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableTarget(t, server, client)
}

func TestSocks4aClientFailure(t *testing.T) {
	t.Parallel()

	// connect unreachable proxy server
	opts := Options{}
	client, err := NewClient("tcp", "localhost:0", &opts)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableProxyServer(t, client)

	// connect unreachable target
	server := testGenerateSocks4aServer(t)
	opts = Options{
		UserID: "admin",
	}
	address := server.Addresses()[0].String()
	client, err = NewClient("tcp", address, &opts)
	require.NoError(t, err)
	testsuite.ProxyClientWithUnreachableTarget(t, server, client)
}
