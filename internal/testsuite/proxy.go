package testsuite

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
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

var (
	httpServer         http.Server
	HTTPServerPort     string
	httpsServer        http.Server
	HTTPSServerPort    string
	httpsCA            *x509.Certificate
	initHTTPServerOnce sync.Once
)

// InitHTTPServers is used to create  http test server
func InitHTTPServers(t testing.TB) {
	initHTTPServerOnce.Do(func() {
		// set handler
		var data = []byte("hello")
		serverMux := http.NewServeMux()
		serverMux.HandleFunc("/t", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			_, _ = w.Write(data)
		})

		// initialize http server
		httpServer.Handler = serverMux

		// initialize https server
		httpsServer.Handler = serverMux
		caASN1, cPEMBlock, cPriPEMBlock := TLSCertificate(t)
		cert, err := tls.X509KeyPair(cPEMBlock, cPriPEMBlock)
		require.NoError(t, err)
		httpsServer.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		// client Root CA
		httpsCA, err = x509.ParseCertificate(caASN1)
		require.NoError(t, err)

		// start HTTP Server Listener
		l1, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		_, HTTPServerPort, err = net.SplitHostPort(l1.Addr().String())
		require.NoError(t, err)
		l2, err := net.Listen("tcp", "[::1]:"+HTTPServerPort)
		require.NoError(t, err)
		// start HTTPS Server Listener
		l3, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		_, HTTPSServerPort, err = net.SplitHostPort(l3.Addr().String())
		require.NoError(t, err)
		l4, err := net.Listen("tcp", "[::1]:"+HTTPSServerPort)
		require.NoError(t, err)
		go func() { _ = httpServer.Serve(l1) }()
		go func() { _ = httpServer.Serve(l2) }()
		go func() { _ = httpsServer.ServeTLS(l3, "", "") }()
		go func() { _ = httpsServer.ServeTLS(l4, "", "") }()
		// print proxy server address
		fmt.Printf("[debug] HTTP Server Port: %s\n", HTTPServerPort)
		fmt.Printf("[debug] HTTPS Server Port: %s\n", HTTPSServerPort)
	})
}

// HTTPClient is used to get target and compare result
func HTTPClient(t testing.TB, transport *http.Transport, hostname string) {
	InitHTTPServers(t)

	// add CA
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = new(tls.Config)
	}
	if transport.TLSClientConfig.RootCAs == nil {
		transport.TLSClientConfig.RootCAs = x509.NewCertPool()
	}
	transport.TLSClientConfig.RootCAs.AddCert(httpsCA)

	// make http client
	client := http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}
	defer client.CloseIdleConnections()

	do := func(req *http.Request) {
		resp, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		defer func() { _ = resp.Body.Close() }()
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "hello", string(b))
	}
	// get http
	url := fmt.Sprintf("http://%s:%s/t", hostname, HTTPServerPort)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	do(req)
	// get https
	url = fmt.Sprintf("https://%s:%s/t", hostname, HTTPSServerPort)
	req, err = http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	do(req)
}

// ProxyServer is used to test proxy server
func ProxyServer(t testing.TB, server io.Closer, transport *http.Transport) {
	defer func() {
		require.NoError(t, server.Close())
		require.NoError(t, server.Close())
		IsDestroyed(t, server)
	}()
	if EnableIPv4() {
		HTTPClient(t, transport, "127.0.0.1")
	}
	if EnableIPv6() {
		HTTPClient(t, transport, "[::1]")
	}
	HTTPClient(t, transport, "localhost")
}

// ProxyConn is used to check proxy client Dial
func ProxyConn(t testing.TB, conn net.Conn) {
	defer func() { _ = conn.Close() }()
	// send http request
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprint(buf, "GET /t HTTP/1.1\r\n")
	_, _ = fmt.Fprint(buf, "Host: localhost\r\n\r\n")
	_, err := io.Copy(conn, buf)
	require.NoError(t, err)

	// get response
	buf.Reset()
	buffer := make([]byte, 1)
	for {
		n, err := conn.Read(buffer)
		require.NoError(t, err)
		buf.Write(buffer[:n])
		if buf.Len() > 4 {
			if bytes.Equal(buf.Bytes()[buf.Len()-4:], []byte("\r\n\r\n")) {
				break
			}
		}
	}

	// read body
	hello := make([]byte, 5)
	_, err = io.ReadFull(conn, hello)
	require.NoError(t, err)
	require.Equal(t, "hello", string(hello))
}

// for test proxy client
type proxyClient interface {
	Dial(network, address string) (net.Conn, error)
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	Connect(ctx context.Context, conn net.Conn, network, address string) (net.Conn, error)
	HTTP(t *http.Transport)
	Timeout() time.Duration
	Server() (network string, address string)
	Info() string
}

// ProxyClient is used to test proxy client
func ProxyClient(t testing.TB, server io.Closer, client proxyClient) {
	InitHTTPServers(t)

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

			address := "127.0.0.1:" + HTTPServerPort
			conn, err := client.Dial(network, address)
			require.NoError(t, err)
			ProxyConn(t, conn)

			conn, err = client.DialContext(context.Background(), network, address)
			require.NoError(t, err)
			ProxyConn(t, conn)

			address = "localhost:" + HTTPServerPort
			conn, err = client.DialTimeout(network, address, 0)
			require.NoError(t, err)
			ProxyConn(t, conn)
		}

		if EnableIPv6() && !strings.Contains(client.Info(), "socks4") {
			const network = "tcp6"

			address := "[::1]:" + HTTPServerPort
			conn, err := client.Dial(network, address)
			require.NoError(t, err)
			ProxyConn(t, conn)

			conn, err = client.DialContext(context.Background(), network, address)
			require.NoError(t, err)
			ProxyConn(t, conn)

			address = "localhost:" + HTTPServerPort
			conn, err = client.DialTimeout(network, address, 0)
			require.NoError(t, err)
			ProxyConn(t, conn)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := new(http.Transport)
		client.HTTP(transport)
		HTTPClient(t, transport, "localhost")
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
	_, err = client.Connect(context.Background(), nil, "foo", "")
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
