package http

import (
	"crypto/tls"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert/certutil"
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
		Password: "123456",
	}
	opts.Server.TLSConfig = *serverCfg
	server, err := NewServer("test", logger.Test, &opts)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	return server, clientCfg
}

func TestHTTPProxyServer(t *testing.T) {
	server := testGenerateHTTPServer(t)
	t.Log("http proxy address:", server.Address())
	t.Log("http proxy info:", server.Info())

	// make client
	u, err := url.Parse("http://admin:123456@" + server.Address())
	require.NoError(t, err)
	transport := &http.Transport{Proxy: http.ProxyURL(u)}
	client := http.Client{
		Transport: transport,
		Timeout:   time.Minute,
	}
	defer client.CloseIdleConnections()

	testsuite.ProxyServer(t, server, &client)
}

func TestHTTPSProxyServer(t *testing.T) {
	server, tlsConfig := testGenerateHTTPSServer(t)
	t.Log("https proxy address:", server.Address())
	t.Log("https proxy info:", server.Info())

	// make client
	u, err := url.Parse("https://admin:123456@" + server.Address())
	require.NoError(t, err)
	transport := &http.Transport{Proxy: http.ProxyURL(u)}
	transport.TLSClientConfig = new(tls.Config)
	require.NoError(t, err)
	// add cert
	transport.TLSClientConfig.RootCAs, err = certutil.SystemCertPool()
	require.NoError(t, err)
	rootCAs, err := tlsConfig.RootCA()
	require.NoError(t, err)
	transport.TLSClientConfig.RootCAs.AddCert(rootCAs[0])
	client := http.Client{
		Transport: transport,
		Timeout:   time.Minute,
	}
	defer client.CloseIdleConnections()

	testsuite.ProxyServer(t, server, &client)
}

func TestAuthenticate(t *testing.T) {
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
	server, err := NewServer("test", logger.Test, nil)
	require.NoError(t, err)
	require.Error(t, server.ListenAndServe("foo", "localhost:0"))
}
