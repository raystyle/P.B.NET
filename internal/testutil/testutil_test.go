package testutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert"
)

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

func TestConn(t *testing.T) {
	server, client := net.Pipe()
	Conn(t, server, client, true)
}

func TestDeployHTTPServer(t *testing.T) {
	// http
	httpServer := http.Server{Addr: "127.0.0.1:0"}
	port := DeployHTTPServer(t, &httpServer, nil)
	t.Log("http server port:", port)
	defer func() { _ = httpServer.Close() }()
	// http client
	client := http.Client{}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/", port))
	require.NoError(t, err)
	_, err = io.Copy(ioutil.Discard, resp.Body)
	require.NoError(t, err)
	client.CloseIdleConnections()

	// https
	certCfg := cert.Config{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	kp, err := cert.Generate(nil, nil, &certCfg)
	require.NoError(t, err)
	httpsServer := http.Server{Addr: "127.0.0.1:0"}
	port = DeployHTTPServer(t, &httpsServer, kp)
	t.Log("https server port:", port)
	defer func() { _ = httpsServer.Close() }()
	// https client
	tlsConfig := tls.Config{RootCAs: x509.NewCertPool()}
	tlsConfig.RootCAs.AddCert(kp.Certificate)
	client = http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tlsConfig,
		},
	}
	resp, err = client.Get(fmt.Sprintf("https://127.0.0.1:%s/", port))
	require.NoError(t, err)
	_, err = io.Copy(ioutil.Discard, resp.Body)
	require.NoError(t, err)
	client.CloseIdleConnections()
}
