package socks

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func testGenerateSocks5Server(t *testing.T) *Server {
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	server, err := NewServer("test", logger.Test, &opts)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	return server
}

func testGenerateSocks4aServer(t *testing.T) *Server {
	opts := Options{
		UserID: "admin",
	}
	server, err := NewServer("test", logger.Test, &opts)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	return server
}

func TestSocks5Server(t *testing.T) {
	t.Parallel()

	server := testGenerateSocks5Server(t)
	t.Log("socks5 address:", server.Address())
	t.Log("socks5 info:", server.Info())

	// make client
	u, err := url.Parse("socks5://admin:123456@" + server.Address())
	require.NoError(t, err)
	transport := http.Transport{Proxy: http.ProxyURL(u)}

	testsuite.ProxyServer(t, server, &transport)
}

func TestSocks4aServer(t *testing.T) {
	t.Parallel()

	opts := Options{}
	server, err := NewServer("test", logger.Test, &opts)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	defer func() {
		require.NoError(t, server.Close())
		require.NoError(t, server.Close())
		testsuite.IsDestroyed(t, server)
	}()
	t.Log("socks4a address:", server.Address())
	t.Log("socks4a info:", server.Info())
	// use firefox to test it, because the http.Client
	// only support socks5, http and https

	// select {}
}

func TestSocks5Authenticate(t *testing.T) {
	t.Parallel()

	server := testGenerateSocks5Server(t)
	defer func() {
		require.NoError(t, server.Close())
		testsuite.IsDestroyed(t, server)
	}()
	opt := Options{
		Username: "admin",
		Password: "123457",
	}
	client, err := NewClient("tcp", server.Address(), &opt)
	require.NoError(t, err)
	_, err = client.Dial("tcp", "localhost:0")
	require.Error(t, err)
}

func TestSocks4aUserID(t *testing.T) {
	t.Parallel()

	server := testGenerateSocks4aServer(t)
	defer func() {
		require.NoError(t, server.Close())
		testsuite.IsDestroyed(t, server)
	}()
	opt := Options{
		UserID: "foo-user-id",
	}
	client, err := NewClient("tcp", server.Address(), &opt)
	require.NoError(t, err)
	_, err = client.Dial("tcp", "localhost:0")
	require.Error(t, err)
}

func TestSocks5ServerWithUnknownNetwork(t *testing.T) {
	t.Parallel()

	server, err := NewServer("test", logger.Test, nil)
	require.NoError(t, err)
	require.Error(t, server.ListenAndServe("foo", "localhost:0"))
}
