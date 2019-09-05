package socks5

import (
	"context"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var address = []string{"github.com:443", "[2606:4700::6810:f9f9]:443", "8.8.8.8:53"}

func TestSocks5Client(t *testing.T) {
	server := testGenerateServer(t)
	err := server.ListenAndServe("localhost:0", 0)
	require.NoError(t, err)
	defer func() {
		err = server.Stop()
		require.NoError(t, err)
	}()
	client, err := NewClient(&Config{
		Network:  "tcp",
		Address:  server.Addr(),
		Username: "admin",
		Password: "123456",
	})
	require.NoError(t, err)
	testSocks5(t, client)
}

func TestSocks5ClientChain(t *testing.T) {
	// server 1
	server1 := testGenerateServer(t)
	err := server1.ListenAndServe("localhost:0", 0)
	require.NoError(t, err)
	defer func() {
		err = server1.Stop()
		require.NoError(t, err)
	}()
	c1 := &Config{
		Network:  "tcp",
		Address:  server1.Addr(),
		Username: "admin",
		Password: "123456",
	}
	// server 2
	server2 := testGenerateServer(t)
	err = server2.ListenAndServe("localhost:0", 0)
	require.NoError(t, err)
	defer func() {
		err = server2.Stop()
		require.NoError(t, err)
	}()
	c2 := &Config{
		Network:  "tcp",
		Address:  server2.Addr(),
		Username: "admin",
		Password: "123456",
	}
	s, err := NewClient(c1, c2)
	require.NoError(t, err)
	testSocks5(t, s)
}

func testSocks5(t *testing.T, c *Client) {
	// test dial
	func() {
		for _, addr := range address {
			conn, err := c.Dial("tcp", addr)
			require.NoError(t, err)
			// t.Log(conn.LocalAddr(), conn.RemoteAddr())
			_ = conn.Close()
			conn, err = c.DialContext(context.Background(), "tcp", addr)
			require.NoError(t, err)
			_ = conn.Close()
			conn, err = c.DialTimeout("tcp", addr, 0)
			require.NoError(t, err)
			_ = conn.Close()
		}
	}()
	// dial failed
	addr := "127.0.0.1:65536"
	_, err := c.DialTimeout("tcp", addr, time.Second)
	require.Error(t, err)
	// test http
	transport := &http.Transport{}
	c.HTTP(transport)
	client := http.Client{
		Transport: transport,
	}
	resp, err := client.Get("https://github.com/")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	t.Log(string(b))
}
