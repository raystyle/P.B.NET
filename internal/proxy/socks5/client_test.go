package socks5

import (
	"context"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var address = []string{"www.baidu.com:443", "[2400:da00:2::29]:443", "8.8.8.8:53"}

func Test_Socks5_Client(t *testing.T) {
	server := test_generate_server(t)
	err := server.Listen_And_Serve(":0", 0)
	require.Nil(t, err, err)
	defer func() {
		err = server.Stop()
		require.Nil(t, err, err)
	}()
	client, err := New_Client(&Config{
		Network:  "tcp",
		Address:  server.Addr(),
		Username: "admin",
		Password: "123456",
	})
	require.Nil(t, err, err)
	test_socks5(t, client)
}

func Test_Socks5_Client_Chain(t *testing.T) {
	// server 1
	server1 := test_generate_server(t)
	err := server1.Listen_And_Serve(":0", 0)
	require.Nil(t, err, err)
	defer func() {
		err = server1.Stop()
		require.Nil(t, err, err)
	}()
	c1 := &Config{
		Network:  "tcp",
		Address:  server1.Addr(),
		Username: "admin",
		Password: "123456",
	}
	// server 2
	server2 := test_generate_server(t)
	err = server2.Listen_And_Serve(":0", 0)
	require.Nil(t, err, err)
	defer func() {
		err = server2.Stop()
		require.Nil(t, err, err)
	}()
	c2 := &Config{
		Network:  "tcp",
		Address:  server2.Addr(),
		Username: "admin",
		Password: "123456",
	}
	s, err := New_Client(c1, c2)
	require.Nil(t, err, err)
	test_socks5(t, s)
}

func test_socks5(t *testing.T, c *Client) {
	// test dial
	func() {
		for _, addr := range address {
			conn, err := c.Dial("tcp", addr)
			require.Nil(t, err, err)
			//t.Log(conn.LocalAddr(), conn.RemoteAddr())
			_ = conn.Close()
			conn, err = c.Dial_Context(context.Background(), "tcp", addr)
			require.Nil(t, err, err)
			_ = conn.Close()
			conn, err = c.Dial_Timeout("tcp", addr, 0)
			require.Nil(t, err, err)
			_ = conn.Close()
		}
	}()
	// dial failed
	addr := "127.0.0.1:65536"
	_, err := c.Dial_Timeout("tcp", addr, time.Second)
	require.NotNil(t, err)
	// test http
	transport := &http.Transport{}
	c.HTTP(transport)
	client := http.Client{
		Transport: transport,
	}
	resp, err := client.Get("https://ip.cn/")
	require.Nil(t, err, err)
	defer func() {
		_ = resp.Body.Close()
	}()
	b, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err, err)
	t.Log(string(b))
}
