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

const (
	// http header User-Agent
	ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:67.0) Gecko/20100101 Firefox/67.0"
	// http header Accept
	accept = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
	// http header Accept-Language
	acceptLanguage = "zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2"
)

var header = make(http.Header)

func init() {
	header.Set("User-Agent", ua)
	header.Set("Accept", accept)
	header.Set("Accept-Language", acceptLanguage)
	header.Set("Connection", "keep-alive")
}

func ProxyServer(t testing.TB, server io.Closer, client *http.Client) {
	defer func() {
		require.NoError(t, server.Close())
		require.NoError(t, server.Close())
		IsDestroyed(t, server)
	}()

	// get http
	req, err := http.NewRequest(http.MethodGet, GetHTTP(), nil)
	require.NoError(t, err)
	req.Header = header.Clone()
	resp, err := client.Do(req)
	require.NoError(t, err)
	HTTPResponse(t, resp)

	// get https
	req, err = http.NewRequest(http.MethodGet, GetHTTPS(), nil)
	require.NoError(t, err)
	req.Header = header.Clone()
	resp, err = client.Do(req)
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
		if EnableIPv4() {
			const network = "tcp4"

			addr := GetIPv4Address()
			conn, err := client.Dial(network, addr)
			require.NoError(t, err)
			_ = conn.Close()

			addr = GetIPv4Address()
			conn, err = client.DialContext(context.Background(), network, addr)
			require.NoError(t, err)
			_ = conn.Close()

			addr = GetIPv4Address()
			conn, err = client.DialTimeout(network, addr, 0)
			require.NoError(t, err)
			_ = conn.Close()
		}

		if EnableIPv6() && !strings.Contains(client.Info(), "socks4") {
			const network = "tcp6"

			addr := GetIPv6Address()
			conn, err := client.Dial(network, addr)
			require.NoError(t, err)
			_ = conn.Close()

			addr = GetIPv6Address()
			conn, err = client.DialContext(context.Background(), network, addr)
			require.NoError(t, err)
			_ = conn.Close()

			addr = GetIPv6Address()
			conn, err = client.DialTimeout(network, addr, 0)
			require.NoError(t, err)
			_ = conn.Close()
		}
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
		req, err := http.NewRequest(http.MethodGet, GetHTTP(), nil)
		require.NoError(t, err)
		req.Header = header.Clone()
		resp, err := client.Do(req)
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
		req, err := http.NewRequest(http.MethodGet, GetHTTPS(), nil)
		require.NoError(t, err)
		req.Header = header.Clone()
		resp, err := client.Do(req)
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
