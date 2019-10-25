package http

import (
	"context"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

func TestClient(t *testing.T) {
	server := testGenerateServer(t)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	defer func() {
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 1)
	}()
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err := NewClient("tcp", server.Address(), false, &opts)
	require.NoError(t, err)
	wg := sync.WaitGroup{}

	// set test address
	var addresses = []string{
		"8.8.8.8:53",
		"cloudflare-dns.com:443",
	}
	if testutil.IPv6() {
		addresses = append(addresses, "[2606:4700::6810:f9f9]:443")
	}

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
		resp, err := client.Get("https://github.com/robots.txt")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "# If you w", string(b)[:10])
		_ = resp.Body.Close()
		transport.CloseIdleConnections()
	}()

	// https
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := &http.Transport{}
		client.HTTP(transport)
		client := http.Client{Transport: transport}
		resp, err := client.Get("https://github.com/robots.txt")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "# If you w", string(b)[:10])
		_ = resp.Body.Close()
		transport.CloseIdleConnections()
	}()

	// http
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := &http.Transport{}
		client.HTTP(transport)
		client := http.Client{Transport: transport}
		resp, err := client.Get("http://www.msftconnecttest.com/connecttest.txt")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "Microsoft Connect Test", string(b))
		_ = resp.Body.Close()
		transport.CloseIdleConnections()
	}()

	wg.Wait()
	t.Log(client.Info())
	testutil.IsDestroyed(t, client, 1)
}
