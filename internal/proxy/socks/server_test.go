package socks

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func testGenerateSocks5Server(t *testing.T) *Server {
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	server, err := NewSocks5Server("test", logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe("tcp", "localhost:0")
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	return server
}

func testGenerateSocks4aServer(t *testing.T) *Server {
	opts := Options{
		UserID: "admin",
	}
	server, err := NewSocks4aServer("test", logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe("tcp", "localhost:0")
		require.NoError(t, err)
	}()
	go func() {
		err := server.ListenAndServe("tcp", "localhost:0")
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	return server
}

func testGenerateSocks4Server(t *testing.T) *Server {
	opts := Options{
		UserID: "admin",
	}
	server, err := NewSocks4Server("test", logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe("tcp", "localhost:0")
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	return server
}

func TestSocks5Server(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks5Server(t)
	addresses := server.Addresses()

	t.Log("socks5 address:", addresses)
	t.Log("socks5 info:", server.Info())

	// make client
	u, err := url.Parse("socks5://admin:123456@" + addresses[0].String())
	require.NoError(t, err)
	transport := http.Transport{Proxy: http.ProxyURL(u)}

	testsuite.ProxyServer(t, server, &transport)
}

func TestSocks4aServer(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks4aServer(t)
	defer func() {
		require.NoError(t, server.Close())
		require.NoError(t, server.Close())
		testsuite.IsDestroyed(t, server)
	}()
	t.Log("socks4a address:", server.Addresses())
	t.Log("socks4a info:", server.Info())

	// use external tool to test it, because the http.Client
	// only support socks5, http and https
	// select {}
}

func TestSocks4Server(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks4Server(t)
	defer func() {
		require.NoError(t, server.Close())
		require.NoError(t, server.Close())
		testsuite.IsDestroyed(t, server)
	}()
	t.Log("socks4 address:", server.Addresses()[0])
	t.Log("socks4 info:", server.Info())

	// use external tool to test it, because the http.Client
	// only support socks5, http and https
	// select {}
}
