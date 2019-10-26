package socks

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

func TestSocks5Client(t *testing.T) {
	server := testGenerateSocks5Server(t)
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err := NewClient("tcp", server.Address(), false, &opts)
	require.NoError(t, err)
	testSocksClient(t, server, client)
}

func TestSocks4Client(t *testing.T) {
	server := testGenerateSocks4aServer(t)
	opts := Options{
		UserID: "admin",
	}
	client, err := NewClient("tcp", server.Address(), true, &opts)
	require.NoError(t, err)
	testSocksClient(t, server, client)
}

func TestSocks5ClientWithoutPassword(t *testing.T) {
	server, err := NewServer("test", logger.Test, false, nil)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	client, err := NewClient("tcp", server.Address(), false, nil)
	require.NoError(t, err)
	testSocksClient(t, server, client)
}

func TestSocks4ClientWithoutUserID(t *testing.T) {
	server, err := NewServer("test", logger.Test, true, nil)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	client, err := NewClient("tcp", server.Address(), true, nil)
	require.NoError(t, err)
	testSocksClient(t, server, client)
}

func testSocksClient(t *testing.T, server *Server, client *Client) {
	defer func() {
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 1)
	}()

	// test target
	var addresses = []string{"8.8.8.8:53", "cloudflare-dns.com:443"}
	if !server.socks4 {
		if testutil.IPv6() {
			addresses = append(addresses, "[2606:4700::6810:f9f9]:443")
		}
	}

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

	// http
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

	// https (don't need)
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

	wg.Add(1)
	go func() {
		defer wg.Done()
		// dial failed
		addr := "127.0.0.1:65536"
		_, err := client.DialTimeout("tcp", addr, time.Second)
		require.Error(t, err)
	}()

	wg.Wait()
	t.Log("client timeout:", client.Timeout())
	network, address := client.Address()
	t.Logf("client address: %s %s", network, address)
	t.Log("client info:", client.Info())
	testutil.IsDestroyed(t, client, 1)
}
