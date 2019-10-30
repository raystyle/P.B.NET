package testsuite

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ProxyServer(t testing.TB, server io.Closer, client *http.Client) {
	defer func() {
		require.NoError(t, server.Close())
		require.NoError(t, server.Close())
		IsDestroyed(t, server)
	}()

	// get https
	resp, err := client.Get(GetHTTPS())
	require.NoError(t, err)
	HTTPResponse(t, resp)

	// get http
	resp, err = client.Get(GetHTTP())
	require.NoError(t, err)
	HTTPResponse(t, resp)
}

// for test proxy client
type proxyClient interface {
	Dial(network, address string) (net.Conn, error)
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	Connect(conn net.Conn, network, address string) (net.Conn, error)
	HTTP(t *http.Transport)
	Timeout() time.Duration
	Server() (network string, address string)
	Info() string
}

// ProxyClient is used to test proxy client
func ProxyClient(t testing.TB, server io.Closer, client proxyClient) {
	defer func() {
		require.NoError(t, server.Close())
		IsDestroyed(t, server)
	}()
	wg := sync.WaitGroup{}

	// test Dial and DialTimeout
	wg.Add(1)
	go func() {
		defer wg.Done()
		var targets []string
		if EnableIPv4() {
			targets = append(targets, GetIPv4Address())
		}
		if EnableIPv6() && !strings.Contains(client.Info(), "socks4") {
			targets = append(targets, GetIPv6Address())
		}
		for _, target := range targets {
			wg.Add(1)
			go func(target string) {
				defer wg.Done()
				conn, err := client.Dial("tcp", target)
				require.NoError(t, err)
				_ = conn.Close()
				conn, err = client.DialTimeout("tcp", target, 0)
				require.NoError(t, err)
				_ = conn.Close()
			}(target)
		}
	}()

	// test DialContext (http)
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := http.Transport{DialContext: client.DialContext}
		client := http.Client{
			Transport: &transport,
			Timeout:   time.Minute,
		}
		defer client.CloseIdleConnections()
		resp, err := client.Get(GetHTTP())
		require.NoError(t, err)
		HTTPResponse(t, resp)
	}()

	// test DialContext (https)
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := http.Transport{DialContext: client.DialContext}
		client := http.Client{
			Transport: &transport,
			Timeout:   time.Minute,
		}
		defer client.CloseIdleConnections()
		resp, err := client.Get(GetHTTPS())
		require.NoError(t, err)
		HTTPResponse(t, resp)
	}()

	// test HTTP with http target
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := &http.Transport{}
		client.HTTP(transport)
		client := http.Client{
			Transport: transport,
			Timeout:   time.Minute,
		}
		defer client.CloseIdleConnections()
		resp, err := client.Get(GetHTTP())
		require.NoError(t, err)
		HTTPResponse(t, resp)
	}()

	// test HTTP with https target
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := &http.Transport{}
		client.HTTP(transport)
		client := http.Client{
			Transport: transport,
			Timeout:   time.Minute,
		}
		defer client.CloseIdleConnections()
		resp, err := client.Get(GetHTTPS())
		require.NoError(t, err)
		HTTPResponse(t, resp)
	}()

	wg.Wait()

	t.Log("timeout:", client.Timeout())
	network, address := client.Server()
	t.Log("server:", network, address)
	t.Log("info:", client.Info())
	IsDestroyed(t, client)
}

// ProxyClientWithUnreachableProxyServer is used to test proxy client that
// can't connect proxy server
func ProxyClientWithUnreachableProxyServer(t testing.TB, client proxyClient) {
	// unknown network
	_, err := client.Dial("foo", "")
	require.Error(t, err)
	t.Log("Dial:\n", err)
	_, err = client.DialContext(context.Background(), "foo", "")
	require.Error(t, err)
	t.Log("DialContext:\n", err)
	_, err = client.DialTimeout("foo", "", time.Second)
	require.Error(t, err)
	t.Log("DialTimeout:\n", err)
	_, err = client.Connect(nil, "foo", "")
	require.Error(t, err)
	t.Log("Connect:\n", err)

	// unreachable proxy server
	_, err = client.Dial("tcp", "")
	require.Error(t, err)
	t.Log("Dial:\n", err)
	_, err = client.DialContext(context.Background(), "tcp", "")
	require.Error(t, err)
	t.Log("DialContext:\n", err)
	_, err = client.DialTimeout("tcp", "", time.Second)
	require.Error(t, err)
	t.Log("DialTimeout:\n", err)
	IsDestroyed(t, client)
}

// ProxyClientWithUnreachableTarget is used to test proxy client that
// connect unreachable target
func ProxyClientWithUnreachableTarget(t testing.TB, server io.Closer, client proxyClient) {
	defer func() {
		require.NoError(t, server.Close())
		IsDestroyed(t, server)
	}()
	const unreachableTarget = "0.0.0.0:1"
	_, err := client.Dial("tcp", unreachableTarget)
	require.Error(t, err)
	t.Log("Dial -> Connect:\n", err)
	_, err = client.DialContext(context.Background(), "tcp", unreachableTarget)
	require.Error(t, err)
	t.Log("DialContext -> Connect:\n", err)
	_, err = client.DialTimeout("tcp", unreachableTarget, time.Second)
	require.Error(t, err)
	t.Log("DialTimeout -> Connect:\n", err)
	IsDestroyed(t, client)
}
