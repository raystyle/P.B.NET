package http

import (
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/options"
	"project/internal/testutil"
)

func testGenerateHTTPServer(t *testing.T) *Server {
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	server, err := NewServer("test", logger.Test, false, &opts)
	require.NoError(t, err)
	return server
}

func testGenerateHTTPSServer(t *testing.T) (*Server, *options.TLSConfig) {
	serverCfg, clientCfg := testutil.TLSConfigOptionPair(t)
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	opts.Server.TLSConfig = *serverCfg
	server, err := NewServer("test", logger.Test, true, &opts)
	require.NoError(t, err)
	return server, clientCfg
}

func TestServer(t *testing.T) {
	// http
	server := testGenerateHTTPServer(t)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	t.Log("address:", server.Address())
	t.Log("info:", server.Info())
	require.NoError(t, server.Close())
	require.NoError(t, server.Close())
	testutil.IsDestroyed(t, server, 1)

	// https
	server, _ = testGenerateHTTPSServer(t)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	t.Log("address:", server.Address())
	t.Log("info:", server.Info())
	require.NoError(t, server.Close())
	require.NoError(t, server.Close())
	testutil.IsDestroyed(t, server, 1)
}

func TestAuthenticate(t *testing.T) {
	server := testGenerateHTTPServer(t)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	defer func() {
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 1)
	}()

	hc := http.Client{}
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

	hc.CloseIdleConnections()

	// invalid username/password
	opts := Options{
		Username: "admin",
		Password: "123457",
	}
	client, err := NewClient("tcp", server.Address(), false, &opts)
	require.NoError(t, err)
	transport := &http.Transport{}
	client.HTTP(transport)
	_, err = (&http.Client{Transport: transport}).Get("https://github.com/")
	require.Error(t, err)
	transport.CloseIdleConnections()
	transport.Proxy = nil
	testutil.IsDestroyed(t, client, 1)
}
