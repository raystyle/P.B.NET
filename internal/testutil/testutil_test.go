package testutil

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIPv6(t *testing.T) {
	IPv6()
}

func TestPPROF(t *testing.T) {
	PPROF()
}

func TestIsDestroyed(t *testing.T) {
	a := 1
	n, err := fmt.Fprintln(ioutil.Discard, a)
	require.Equal(t, n, 2)
	require.NoError(t, err)
	if !isDestroyed(&a, 1) {
		t.Fatal("doesn't destroyed")
	}

	b := 2
	if isDestroyed(&b, 1) {
		t.Fatal("destroyed")
	}
	n, err = fmt.Fprintln(ioutil.Discard, b)
	require.Equal(t, n, 2)
	require.NoError(t, err)

	c := 3
	n, err = fmt.Fprintln(ioutil.Discard, c)
	require.Equal(t, n, 2)
	require.NoError(t, err)
	IsDestroyed(t, &c, 1)
}

func TestTLSConfigPair(t *testing.T) {
	serverCfg, clientCfg := TLSConfigPair(t)
	listener, err := tls.Listen("tcp", "localhost:0", serverCfg)
	require.NoError(t, err)
	var server net.Conn
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		server, err = listener.Accept()
		require.NoError(t, err)
		// must Handshake
		require.NoError(t, server.(*tls.Conn).Handshake())
	}()
	client, err := tls.Dial("tcp", listener.Addr().String(), clientCfg)
	require.NoError(t, err)
	wg.Wait()
	Conn(t, server, client, true)
}

func TestListenerAndDial(t *testing.T) {
	l, err := net.Listen("tcp4", "localhost:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	ListenerAndDial(t, l, func() (net.Conn, error) {
		return net.Dial("tcp4", addr)
	}, true)

	if IPv6() {
		l, err = net.Listen("tcp6", "localhost:0")
		require.NoError(t, err)
		addr = l.Addr().String()
		ListenerAndDial(t, l, func() (net.Conn, error) {
			return net.Dial("tcp6", addr)
		}, true)
	}
}

func TestConn(t *testing.T) {
	server, client := net.Pipe()
	Conn(t, server, client, true)
}

func TestHTTPServer(t *testing.T) {
	// http
	httpServer := http.Server{Addr: "localhost:0"}
	port := RunHTTPServer(t, "tcp", &httpServer)
	defer func() { _ = httpServer.Close() }()
	t.Log("http server port:", port)
	client := http.Client{}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%s/", port))
	require.NoError(t, err)
	_, err = io.Copy(ioutil.Discard, resp.Body)
	require.NoError(t, err)
	client.CloseIdleConnections()

	// https
	serverCfg, clientCfg := TLSConfigPair(t)
	httpsServer := http.Server{
		Addr:      "localhost:0",
		TLSConfig: serverCfg,
	}
	port = RunHTTPServer(t, "tcp", &httpsServer)
	defer func() { _ = httpsServer.Close() }()
	t.Log("https server port:", port)
	client = http.Client{
		Transport: &http.Transport{
			TLSClientConfig: clientCfg,
		},
	}
	resp, err = client.Get(fmt.Sprintf("https://localhost:%s/", port))
	require.NoError(t, err)
	_, err = io.Copy(ioutil.Discard, resp.Body)
	require.NoError(t, err)
	client.CloseIdleConnections()
}
