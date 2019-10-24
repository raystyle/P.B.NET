package http

import (
	"context"
	"io"
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
		transport := &http.Transport{DialContext: client.DialContext}
		resp, err := (&http.Client{Transport: transport}).Get("https://github.com/")
		require.NoError(t, err)
		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		_ = resp.Body.Close()
		transport.CloseIdleConnections()
	}()

	t.Log(client.Info())
	wg.Wait()
	testutil.IsDestroyed(t, client, 1)
}
