package testsuite

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsDestroyed(t *testing.T) {
	a := 1
	n, err := fmt.Fprintln(ioutil.Discard, a)
	require.Equal(t, n, 2)
	require.NoError(t, err)
	if !Destroyed(&a) {
		t.Fatal("doesn't destroyed")
	}

	b := 2
	if Destroyed(&b) {
		t.Fatal("destroyed")
	}
	n, err = fmt.Fprintln(ioutil.Discard, b)
	require.Equal(t, n, 2)
	require.NoError(t, err)

	c := 3
	n, err = fmt.Fprintln(ioutil.Discard, c)
	require.Equal(t, n, 2)
	require.NoError(t, err)
	IsDestroyed(t, &c)
}

func TestHTTPServer(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

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
