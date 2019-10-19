package http

import (
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testutil"
)

func TestServer(t *testing.T) {
	server := testGenerateServer(t)
	require.NoError(t, server.ListenAndServe("localhost:0"))
	t.Log("address:", server.Address())
	t.Log("info:", server.Info())
	require.NoError(t, server.Close())
	require.NoError(t, server.Close())
	testutil.IsDestroyed(t, server, 2)
}

func testGenerateServer(t *testing.T) *Server {
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	server, err := NewServer("test", logger.Test, &opts)
	require.NoError(t, err)
	return server
}

func TestAuthenticate(t *testing.T) {
	server := testGenerateServer(t)
	require.NoError(t, server.ListenAndServe("localhost:0"))
	defer func() {
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 2)
	}()
	// no auth method
	resp, err := http.Get("http://" + server.Address())
	require.NoError(t, err)
	_, err = io.Copy(ioutil.Discard, resp.Body)
	require.NoError(t, err)
	// not support method
	req, err := http.NewRequest(http.MethodGet, "http://"+server.Address(), nil)
	require.NoError(t, err)
	req.Header.Set("Proxy-Authorization", "method not-support")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	_, err = io.Copy(ioutil.Discard, resp.Body)
	require.NoError(t, err)
	// invalid username/password
	client, err := NewClient("http://admin:123457@" + server.Address())
	require.NoError(t, err)
	transport := &http.Transport{}
	client.HTTP(transport)
	_, err = (&http.Client{Transport: transport}).Get("https://github.com/")
	require.Error(t, err)
	transport.CloseIdleConnections()
	transport.Proxy = nil
	testutil.IsDestroyed(t, client, 2)
}
