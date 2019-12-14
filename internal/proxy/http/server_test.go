package http

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/options"
	"project/internal/testsuite"
)

func testGenerateHTTPServer(t *testing.T) *Server {
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	server, err := NewServer("test", logger.Test, &opts)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	return server
}

func testGenerateHTTPSServer(t *testing.T) (*Server, *options.TLSConfig) {
	serverCfg, clientCfg := testsuite.TLSConfigOptionPair(t)
	opts := Options{
		HTTPS:    true,
		Username: "admin",
	}
	opts.Server.TLSConfig = *serverCfg
	server, err := NewServer("test", logger.Test, &opts)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	return server, clientCfg
}

func TestHTTPProxyServer(t *testing.T) {
	t.Parallel()

	server := testGenerateHTTPServer(t)
	t.Log("http proxy address:", server.Address())
	t.Log("http proxy info:", server.Info())

	// make client
	u, err := url.Parse("http://admin:123456@" + server.Address())
	require.NoError(t, err)
	transport := http.Transport{Proxy: http.ProxyURL(u)}

	testsuite.ProxyServer(t, server, &transport)
}

func TestHTTPSProxyServer(t *testing.T) {
	t.Parallel()

	server, tlsConfig := testGenerateHTTPSServer(t)
	t.Log("https proxy address:", server.Address())
	t.Log("https proxy info:", server.Info())

	// make client
	u, err := url.Parse("https://admin@" + server.Address())
	require.NoError(t, err)
	transport := http.Transport{Proxy: http.ProxyURL(u)}
	transport.TLSClientConfig, err = tlsConfig.Apply()
	require.NoError(t, err)

	testsuite.ProxyServer(t, server, &transport)
}

func TestAuthenticate(t *testing.T) {
	t.Parallel()

	server := testGenerateHTTPServer(t)
	defer func() {
		require.NoError(t, server.Close())
		testsuite.IsDestroyed(t, server)
	}()

	hc := http.Client{}
	defer hc.CloseIdleConnections()
	// no auth method
	resp, err := hc.Get("http://" + server.Address())
	require.NoError(t, err)
	_, err = io.Copy(ioutil.Discard, resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()

	// not support method
	req, err := http.NewRequest(http.MethodGet, "http://"+server.Address(), nil)
	require.NoError(t, err)
	req.Header.Set("Proxy-Authorization", "method not-support")
	resp, err = hc.Do(req)
	require.NoError(t, err)
	_, err = io.Copy(ioutil.Discard, resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()

	// invalid username/password
	opts := Options{
		Username: "admin",
		Password: "123457",
	}
	client, err := NewClient("tcp", server.Address(), &opts)
	require.NoError(t, err)
	_, err = client.Dial("tcp", "localhost:0")
	require.Error(t, err)
}

func TestHTTPServerWithUnknownNetwork(t *testing.T) {
	t.Parallel()

	server, err := NewServer("test", logger.Test, nil)
	require.NoError(t, err)
	require.Error(t, server.ListenAndServe("foo", "localhost:0"))
}
