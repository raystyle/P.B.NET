package socks

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testutil"
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
		Socks4: true,
		UserID: "admin",
	}
	server, err := NewServer("test", logger.Test, &opts)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	return server
}

func TestSocks5Server(t *testing.T) {
	server := testGenerateSocks5Server(t)
	defer func() {
		require.NoError(t, server.Close())
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 1)
	}()
	t.Log("socks5 address:", server.Address())
	t.Log("socks5 info:", server.Info())

	// make client
	u, err := url.Parse("socks5://admin:123456@" + server.Address())
	require.NoError(t, err)
	transport := &http.Transport{Proxy: http.ProxyURL(u)}
	client := http.Client{Transport: transport}
	defer client.CloseIdleConnections()

	// get https
	resp, err := client.Get("https://github.com/robots.txt")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "# If you w", string(b)[:10])

	// get http
	resp, err = client.Get("http://www.msftconnecttest.com/connecttest.txt")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	b, err = ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Microsoft Connect Test", string(b))
}

func TestSocks4aServer(t *testing.T) {
	opts := Options{Socks4: true}
	server, err := NewServer("test", logger.Test, &opts)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	defer func() {
		require.NoError(t, server.Close())
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 1)
	}()
	t.Log("socks4a address:", server.Address())
	t.Log("socks4a info:", server.Info())
	// use firefox to test it, because http.Client
	// only support socks5, http, https

	// select {}
}

func TestSocks5Authenticate(t *testing.T) {
	server := testGenerateSocks5Server(t)
	defer func() {
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 1)
	}()
	opt := Options{
		Username: "admin",
		Password: "123457",
	}
	client, err := NewClient("tcp", server.Address(), &opt)
	require.NoError(t, err)
	_, err = client.Dial("tcp", "github.com:443")
	require.Error(t, err)
}

func TestSocks4aUserID(t *testing.T) {
	server := testGenerateSocks4aServer(t)
	defer func() {
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 1)
	}()
	opt := Options{
		Socks4: true,
		UserID: "invalid user id",
	}
	client, err := NewClient("tcp", server.Address(), &opt)
	require.NoError(t, err)
	_, err = client.Dial("tcp", "github.com:443")
	require.Error(t, err)
}
