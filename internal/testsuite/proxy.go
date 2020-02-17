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

// HTTPServerPort is the test HTTP server port,
// some test in internal/proxy need it
var (
	HTTPServerPort  string
	HTTPSServerPort string
)

var (
	httpServer  http.Server
	httpsServer http.Server

	// Root CA about server side
	httpsServerCA *x509.Certificate

	// certificate about client side
	httpsClientCert tls.Certificate

	initHTTPServerOnce sync.Once
)

// InitHTTPServers is used to create  http test server
func InitHTTPServers(t testing.TB) {
	initHTTPServerOnce.Do(func() { initHTTPServers(t) })
}

func initHTTPServers(t testing.TB) {
	// set handler
	var data = []byte("hello")
	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/t", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})

	// initialize HTTP server
	httpServer.Handler = serverMux

	// initialize HTTPS server
	httpsServer.Handler = serverMux

	// server side certificate
	caASN1, certPEMBlock, keyPEMBlock := TLSCertificate(t)
	serverCert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	require.NoError(t, err)
	// require client certificate
	httpsServer.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	httpsServerCA, err = x509.ParseCertificate(caASN1)
	require.NoError(t, err)

	// client side certificate
	caASN1, certPEMBlock, keyPEMBlock = TLSCertificate(t)
	httpsClientCert, err = tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	require.NoError(t, err)
	httpsServer.TLSConfig.ClientCAs = x509.NewCertPool()
	httpsClientCA, err := x509.ParseCertificate(caASN1)
	require.NoError(t, err)
	httpsServer.TLSConfig.ClientCAs.AddCert(httpsClientCA)

	// start HTTP and HTTPS Server Listeners
	var (
		l1 net.Listener // HTTP  IPv4
		l2 net.Listener // HTTPS IPv4
		l3 net.Listener // HTTP  IPv6
		l4 net.Listener // HTTPS IPv6
	)

	if IPv4Enabled {
		l1, err = net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		_, HTTPServerPort, err = net.SplitHostPort(l1.Addr().String())
		require.NoError(t, err)

		l2, err = net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		_, HTTPSServerPort, err = net.SplitHostPort(l2.Addr().String())
		require.NoError(t, err)

		go func() { _ = httpServer.Serve(l1) }()
		go func() { _ = httpsServer.ServeTLS(l2, "", "") }()
	}

	if IPv6Enabled {
		if HTTPServerPort != "" {
			l3, err = net.Listen("tcp", "[::1]:"+HTTPServerPort)
			require.NoError(t, err)

			l4, err = net.Listen("tcp", "[::1]:"+HTTPSServerPort)
			require.NoError(t, err)
		} else { // IPv6 Only
			l3, err = net.Listen("tcp", "[::1]:0")
			require.NoError(t, err)
			_, HTTPServerPort, err = net.SplitHostPort(l3.Addr().String())
			require.NoError(t, err)

			l4, err = net.Listen("tcp", "[::1]:0")
			require.NoError(t, err)
			_, HTTPSServerPort, err = net.SplitHostPort(l4.Addr().String())
			require.NoError(t, err)
		}

		go func() { _ = httpServer.Serve(l3) }()
		go func() { _ = httpsServer.ServeTLS(l4, "", "") }()
	}
	// wait go func()
	time.Sleep(250 * time.Millisecond)

	// print proxy server addresses
	fmt.Printf("[debug] HTTP Server Port:  %s\n", HTTPServerPort)
	fmt.Printf("[debug] HTTPS Server Port: %s\n", HTTPSServerPort)
}

// HTTPClient is used to get target and compare result
func HTTPClient(t testing.TB, transport *http.Transport, hostname string) {
	InitHTTPServers(t)

	// add certificate
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = new(tls.Config)
	}
	if transport.TLSClientConfig.RootCAs == nil {
		transport.TLSClientConfig.RootCAs = x509.NewCertPool()
	}
	transport.TLSClientConfig.RootCAs.AddCert(httpsServerCA)
	tlsCerts := transport.TLSClientConfig.Certificates
	transport.TLSClientConfig.Certificates = append(tlsCerts, httpsClientCert)

	// make http client
	client := http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}
	defer client.CloseIdleConnections()

	wg := sync.WaitGroup{}

	do := func(req *http.Request) {
		defer wg.Done()
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "hello", string(b))
	}

	// http
	url := fmt.Sprintf("http://%s:%s/t", hostname, HTTPServerPort)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	wg.Add(1)
	go do(req)

	// https
	url = fmt.Sprintf("https://%s:%s/t", hostname, HTTPSServerPort)
	req, err = http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	wg.Add(1)
	go do(req)

	wg.Wait()
}

// NopCloser is a nop closer
type NopCloser struct {
	padding string
}

// Get is a padding method
func (nc *NopCloser) Get() string {
	return nc.padding
}

// Close is a padding method
func (nc *NopCloser) Close() error {
	nc.padding = "padding"
	return nil
}

// NewNopCloser is used to create a nop closer for ProxyServer
// but only has transport
func NewNopCloser() *NopCloser {
	return new(NopCloser)
}

// ProxyServer is used to test proxy server
func ProxyServer(t testing.TB, server io.Closer, transport *http.Transport) {
	if IPv4Enabled {
		HTTPClient(t, transport, "127.0.0.1")
	}
	if IPv6Enabled {
		HTTPClient(t, transport, "[::1]")
	}
	HTTPClient(t, transport, "localhost")

	require.NoError(t, server.Close())
	require.NoError(t, server.Close())
	IsDestroyed(t, server)
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

	wg := sync.WaitGroup{}

	// Dial
	if IPv4Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			const network = "tcp4"

			address := "127.0.0.1:" + HTTPServerPort
			conn, err := client.Dial(network, address)
			require.NoError(t, err)
			ProxyConn(t, conn)

			conn, err = client.DialContext(context.Background(), network, address)
			require.NoError(t, err)
			ProxyConn(t, conn)

			conn, err = client.DialTimeout(network, address, 0)
			require.NoError(t, err)
			ProxyConn(t, conn)

			// except socks4
			if !strings.Contains(client.Info(), "socks4 ") {
				address = "localhost:" + HTTPServerPort
				conn, err = client.DialTimeout(network, address, 0)
				require.NoError(t, err)
				ProxyConn(t, conn)
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				transport := new(http.Transport)
				client.HTTP(transport)
				HTTPClient(t, transport, "127.0.0.1")
				client.HTTP(transport)
				HTTPClient(t, transport, "127.0.0.1")
			}()
		}()
	}

	// except socks4a, socks4
	if IPv6Enabled && !strings.Contains(client.Info(), "socks4") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			const network = "tcp6"

			address := "[::1]:" + HTTPServerPort
			conn, err := client.Dial(network, address)
			require.NoError(t, err)
			ProxyConn(t, conn)

			conn, err = client.DialContext(context.Background(), network, address)
			require.NoError(t, err)
			ProxyConn(t, conn)

			conn, err = client.DialTimeout(network, address, 0)
			require.NoError(t, err)
			ProxyConn(t, conn)

			address = "localhost:" + HTTPServerPort
			conn, err = client.DialTimeout(network, address, 0)
			require.NoError(t, err)
			ProxyConn(t, conn)

			wg.Add(1)
			go func() {
				defer wg.Done()
				transport := new(http.Transport)
				client.HTTP(transport)
				HTTPClient(t, transport, "[::1]")
				client.HTTP(transport)
				HTTPClient(t, transport, "[::1]")
			}()
		}()
	}

	// HTTP
	if !strings.Contains(client.Info(), "socks4 ") {
		wg.Add(1)
		go func() {
			defer wg.Done() // twice
			transport := new(http.Transport)
			client.HTTP(transport)
			HTTPClient(t, transport, "localhost")
			client.HTTP(transport)
			HTTPClient(t, transport, "localhost")
		}()
	}

	wg.Wait()

	t.Log("timeout:", client.Timeout())
	network, address := client.Server()
	t.Log("server:", network, address)
	t.Log("info:", client.Info())

	IsDestroyed(t, client)
	require.NoError(t, server.Close())
	IsDestroyed(t, server)
}

// ProxyClientCancelConnect is used to cancel proxy client Connect()
func ProxyClientCancelConnect(t testing.TB, server io.Closer, client proxyClient) {
	InitHTTPServers(t)

	conn, err := net.Dial(client.Server())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// cancel
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		cancel()
	}()
	time.Sleep(10 * time.Millisecond)

	address := "127.0.0.1:" + HTTPServerPort
	_, err = client.Connect(ctx, conn, "tcp", address)
	require.Error(t, err)
	wg.Wait()

	IsDestroyed(t, client)
	require.NoError(t, server.Close())
	IsDestroyed(t, server)
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
	require.NoError(t, server.Close())
	IsDestroyed(t, server)
}
