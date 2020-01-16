package http

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/option"
	"project/internal/testsuite"
)

func testGenerateHTTPProxyServer(t *testing.T) *Server {
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	server, err := NewHTTPServer("test", logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe("tcp", "localhost:0")
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	return server
}

func testGenerateHTTPSProxyServer(t *testing.T) (*Server, *option.TLSConfig) {
	serverCfg, clientCfg := testsuite.TLSConfigOptionPair(t)
	opts := Options{
		Username: "admin",
	}
	opts.Server.TLSConfig = *serverCfg
	server, err := NewHTTPSServer("test", logger.Test, &opts)
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
	return server, clientCfg
}

func TestHTTPProxyServer(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateHTTPProxyServer(t)
	addresses := server.Addresses()

	t.Log("http proxy address:\n", addresses)
	t.Log("http proxy info:\n", server.Info())

	// make client
	u, err := url.Parse("http://admin:123456@" + addresses[0].String())
	require.NoError(t, err)
	transport := http.Transport{Proxy: http.ProxyURL(u)}

	testsuite.ProxyServer(t, server, &transport)
}

func TestHTTPSProxyServer(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, tlsConfig := testGenerateHTTPSProxyServer(t)
	addresses := server.Addresses()

	t.Log("https proxy address:\n", addresses)
	t.Log("https proxy info:\n", server.Info())

	// make client
	proxyURL, err := url.Parse("https://admin@" + addresses[1].String())
	require.NoError(t, err)
	transport := http.Transport{Proxy: http.ProxyURL(proxyURL)}
	transport.TLSClientConfig, err = tlsConfig.Apply()
	require.NoError(t, err)

	testsuite.ProxyServer(t, server, &transport)
}

func TestAuthenticate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateHTTPProxyServer(t)
	defer func() {
		require.NoError(t, server.Close())
		testsuite.IsDestroyed(t, server)
	}()
	address := server.Addresses()[0].String()

	client := http.Client{}
	defer client.CloseIdleConnections()

	t.Run("no authenticate method", func(t *testing.T) {
		resp, err := client.Get("http://" + address)
		require.NoError(t, err)
		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("unsupported authenticate method", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		req.Header.Set("Proxy-Authorization", "method not-support")
		resp, err := client.Do(req)
		require.NoError(t, err)
		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("invalid username/password", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		userInfo := url.UserPassword("admin1", "123")
		req.Header.Set("Proxy-Authorization", "Basic "+userInfo.String())
		resp, err := client.Do(req)
		require.NoError(t, err)
		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})
}

func TestServer_ListenAndServe(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewHTTPServer("test", logger.Test, nil)
	require.NoError(t, err)
	require.Error(t, server.ListenAndServe("foo", "localhost:0"))
	require.NoError(t, server.Close())
	testsuite.IsDestroyed(t, server)
}

func TestServer_Close(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewHTTPSServer("test", logger.Test, nil)
	require.NoError(t, err)
	require.NoError(t, server.Close())
	testsuite.IsDestroyed(t, server)
}
