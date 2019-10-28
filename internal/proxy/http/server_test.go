package http

import (
	"crypto/tls"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert"
	"project/internal/logger"
	"project/internal/options"
	"project/internal/testutil"
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
	serverCfg, clientCfg := testutil.TLSConfigOptionPair(t)
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
	// deploy proxy server
	httpServer := testGenerateHTTPServer(t)
	defer func() {
		require.NoError(t, httpServer.Close())
		require.NoError(t, httpServer.Close())
		testutil.IsDestroyed(t, httpServer, 1)
	}()
	t.Log("http server address:", httpServer.Address())
	t.Log("http server info:", httpServer.Info())

	// make client
	u, err := url.Parse("http://admin:123456@" + httpServer.Address())
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

func TestHTTPSProxyServer(t *testing.T) {
	// deploy proxy server
	httpsServer, tlsConfig := testGenerateHTTPSServer(t)
	defer func() {
		require.NoError(t, httpsServer.Close())
		require.NoError(t, httpsServer.Close())
		testutil.IsDestroyed(t, httpsServer, 1)
	}()
	t.Log("https server address:", httpsServer.Address())
	t.Log("https server info:", httpsServer.Info())

	// make client
	u, err := url.Parse("https://admin:123456@" + httpsServer.Address())
	require.NoError(t, err)
	transport := &http.Transport{Proxy: http.ProxyURL(u)}
	transport.TLSClientConfig = new(tls.Config)
	require.NoError(t, err)
	// add cert
	transport.TLSClientConfig.RootCAs, err = cert.SystemCertPool()
	require.NoError(t, err)
	rootCAs, err := tlsConfig.RootCA()
	require.NoError(t, err)
	transport.TLSClientConfig.RootCAs.AddCert(rootCAs[0])
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

func TestAuthenticate(t *testing.T) {
	server := testGenerateHTTPServer(t)
	defer func() {
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 1)
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
	transport := &http.Transport{}
	client.HTTP(transport)
	_, err = (&http.Client{Transport: transport}).Get("https://github.com/")
	require.Error(t, err)
	transport.CloseIdleConnections()
	transport.Proxy = nil
	testutil.IsDestroyed(t, client, 1)
}
