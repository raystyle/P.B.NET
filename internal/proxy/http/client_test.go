package http

import (
	"context"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testutil"
)

var addresses = []string{"8.8.8.8:53", "cloudflare-dns.com:443"}

func init() {
	if testutil.IPv6() {
		addresses = append(addresses, "[2606:4700::6810:f9f9]:443")
	}
}

func TestHTTPProxyClient(t *testing.T) {
	server := testGenerateHTTPServer(t)
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err := NewClient("tcp", server.Address(), &opts)
	require.NoError(t, err)
	testHTTPProxyClient(t, server, client)
}

func TestHTTPSProxyClient(t *testing.T) {
	server, tlsConfig := testGenerateHTTPSServer(t)
	opts := Options{
		HTTPS:     true,
		Username:  "admin",
		Password:  "123456",
		TLSConfig: *tlsConfig,
	}
	client, err := NewClient("tcp", server.Address(), &opts)
	require.NoError(t, err)
	testHTTPProxyClient(t, server, client)
}

func TestHTTPProxyClientWithoutPassword(t *testing.T) {
	server, err := NewServer("test", logger.Test, nil)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	client, err := NewClient("tcp", server.Address(), nil)
	require.NoError(t, err)
	testHTTPProxyClient(t, server, client)
}

func testHTTPProxyClient(t *testing.T, server *Server, client *Client) {
	defer func() {
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 1)
	}()
	wg := sync.WaitGroup{}
	for _, address := range addresses {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			conn, err := client.Dial("tcp", address)
			require.NoError(t, err)
			_ = conn.Close()
			conn, err = client.DialContext(context.Background(), "tcp", address)
			require.NoError(t, err)
			_ = conn.Close()
			conn, err = client.DialTimeout("tcp", address, 0)
			require.NoError(t, err)
			_ = conn.Close()
		}(address)
	}

	// set DialContext
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := http.Transport{DialContext: client.DialContext}
		client := http.Client{Transport: &transport}
		defer client.CloseIdleConnections()
		resp, err := client.Get("https://github.com/robots.txt")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		defer func() { _ = resp.Body.Close() }()
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "# If you w", string(b)[:10])
	}()

	// https
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := &http.Transport{}
		client.HTTP(transport)
		client := http.Client{Transport: transport}
		defer client.CloseIdleConnections()
		resp, err := client.Get("https://github.com/robots.txt")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "# If you w", string(b)[:10])
	}()

	// http
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := &http.Transport{}
		client.HTTP(transport)
		client := http.Client{Transport: transport}
		defer client.CloseIdleConnections()
		resp, err := client.Get("http://www.msftconnecttest.com/connecttest.txt")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "Microsoft Connect Test", string(b))
	}()

	wg.Wait()
	t.Log("client timeout:", client.Timeout())
	t.Log("client info:", client.Info())
	testutil.IsDestroyed(t, client, 1)
}

func TestClientFailedToDialAndConnect(t *testing.T) {
	// dial unreachable proxy server
	client, err := NewClient("tcp", "localhost:0", nil)
	require.NoError(t, err)
	_, err = client.Dial("", "")
	require.Error(t, err)
	t.Log("Dial:\n", err)
	_, err = client.DialContext(context.Background(), "", "")
	require.Error(t, err)
	t.Log("DialContext:\n", err)
	_, err = client.DialTimeout("", "", time.Second)
	require.Error(t, err)
	t.Log("DialTimeout:\n", err)
	testutil.IsDestroyed(t, client, 1)

	// connect unreachable target
	server := testGenerateHTTPServer(t)
	defer func() {
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 1)
	}()
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err = NewClient("tcp", server.Address(), &opts)
	require.NoError(t, err)
	const unreachableTarget = "0.0.0.0:1"
	_, err = client.Dial("", unreachableTarget)
	require.Error(t, err)
	t.Log("Dial -> Connect:\n", err)
	_, err = client.DialContext(context.Background(), "", unreachableTarget)
	require.Error(t, err)
	t.Log("DialContext -> Connect:\n", err)
	_, err = client.DialTimeout("", unreachableTarget, time.Second)
	require.Error(t, err)
	t.Log("DialTimeout -> Connect:\n", err)
	testutil.IsDestroyed(t, client, 1)
}
