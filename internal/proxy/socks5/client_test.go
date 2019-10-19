package socks5

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

var address = []string{
	"cloudflare-dns.com:443",
	"[2606:4700::6810:f9f9]:443",
	"8.8.8.8:53",
}

func TestSocks5Client(t *testing.T) {
	server := testGenerateServer(t)
	require.NoError(t, server.ListenAndServe("localhost:0"))
	defer func() {
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 2)
	}()
	client, err := NewClient(&Config{
		Network:  "tcp",
		Address:  server.Address(),
		Username: "admin",
		Password: "123456",
	})
	require.NoError(t, err)
	testSocks5Client(t, client)
	testutil.IsDestroyed(t, client, 2)
}

func TestSocks5ClientChain(t *testing.T) {
	server1 := testGenerateServer(t)
	require.NoError(t, server1.ListenAndServe("localhost:0"))
	defer func() {
		require.NoError(t, server1.Close())
		testutil.IsDestroyed(t, server1, 2)
	}()
	server2 := testGenerateServer(t)
	require.NoError(t, server2.ListenAndServe("localhost:0"))
	defer func() {
		require.NoError(t, server2.Close())
		testutil.IsDestroyed(t, server2, 2)
	}()
	c1 := &Config{
		Network:  "tcp",
		Address:  server1.Address(),
		Username: "admin",
		Password: "123456",
	}
	c2 := &Config{
		Network:  "tcp",
		Address:  server2.Address(),
		Username: "admin",
		Password: "123456",
	}
	client, err := NewClient(c1, c2)
	require.NoError(t, err)
	testSocks5Client(t, client)
	testutil.IsDestroyed(t, client, 2)
}

func testSocks5Client(t *testing.T, client *Client) {
	// test dial
	func() {
		for _, addr := range address {
			conn, err := client.Dial("tcp", addr)
			require.NoError(t, err)
			_ = conn.Close()
			conn, err = client.DialContext(context.Background(), "tcp", addr)
			require.NoError(t, err)
			_ = conn.Close()
			conn, err = client.DialTimeout("tcp", addr, 0)
			require.NoError(t, err)
			_ = conn.Close()
		}
	}()
	// dial failed
	addr := "127.0.0.1:65536"
	_, err := client.DialTimeout("tcp", addr, time.Second)
	require.Error(t, err)
	// test http
	transport := &http.Transport{}
	client.HTTP(transport)
	resp, err := (&http.Client{Transport: transport}).Get("https://cloudflare-dns.com/")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	_, err = io.Copy(ioutil.Discard, resp.Body)
	require.NoError(t, err)
}
