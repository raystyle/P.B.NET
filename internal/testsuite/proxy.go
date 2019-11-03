package testsuite

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type nopCloser struct {
	padding string
}

func (nc *nopCloser) Get() string {
	return nc.padding
}

func (nc *nopCloser) Close() error {
	nc.padding = "padding"
	return nil
}

func NopCloser() *nopCloser {
	return new(nopCloser)
}

// HTTPClient is used to get target and compare result
func HTTPClient(t testing.TB, client *http.Client, url string) {
	const (
		// http header User-Agent
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:67.0) Gecko/20100101 Firefox/67.0"
		// http header Accept
		accept = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
		// http header Accept-Language
		acceptLanguage = "zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2"
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header = make(http.Header)
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", acceptLanguage)
	req.Header.Set("Connection", "keep-alive")
	resp, err := client.Do(req)
	require.NoError(t, err)

	// compare response
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer func() { _ = resp.Body.Close() }()
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "callback", string(b)[:8])
}

// ProxyServer is used to test proxy server
func ProxyServer(t testing.TB, server io.Closer, client *http.Client) {
	defer func() {
		require.NoError(t, server.Close())
		require.NoError(t, server.Close())
		IsDestroyed(t, server)
	}()

	const format = "http://%s" + suffix

	if EnableIPv4() {
		HTTPClient(t, client, fmt.Sprintf(format, GetIPv4Address()))
	}

	if EnableIPv6() {
		HTTPClient(t, client, fmt.Sprintf(format, GetIPv6Address()))
	}

	// get http
	HTTPClient(t, client, GetHTTP())
	HTTPClient(t, client, GetHTTPS())
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

			addr = GetIPv4Domain()
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

			addr = GetIPv6Domain()
			conn, err = client.DialTimeout(network, addr, 0)
			require.NoError(t, err)
			_ = conn.Close()
		}
	}()

	makeHTTPClient := func() *http.Client {
		transport := &http.Transport{}
		client.HTTP(transport)
		return &http.Client{
			Transport: transport,
			Timeout:   time.Minute,
		}
	}

	// test HTTP with http target
	wg.Add(1)
	go func() {
		defer wg.Done()
		client := makeHTTPClient()
		defer client.CloseIdleConnections()
		HTTPClient(t, client, GetHTTP())
	}()

	// test HTTP with https target
	wg.Add(1)
	go func() {
		defer wg.Done()
		client := makeHTTPClient()
		defer client.CloseIdleConnections()
		HTTPClient(t, client, GetHTTPS())
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
