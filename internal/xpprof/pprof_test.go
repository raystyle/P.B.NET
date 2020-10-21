package xpprof

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/option"
	"project/internal/testsuite"
	"project/internal/testsuite/testtls"
)

const (
	testNetwork = "tcp"
	testAddress = "localhost:0"
)

func testGenerateHTTPServer(t *testing.T) *Server {
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	server, err := NewHTTPServer(logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	testsuite.WaitProxyServerServe(t, server, 1)
	return server
}

func testGenerateHTTPSServer(t *testing.T) (*Server, option.TLSConfig) {
	serverCfg, clientCfg := testtls.OptionPair(t, "127.0.0.1")
	opts := Options{
		Username: "admin",
	}
	opts.Server.TLSConfig = serverCfg
	server, err := NewHTTPSServer(logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	testsuite.WaitProxyServerServe(t, server, 2)
	return server, clientCfg
}

func testFetch(t *testing.T, url string, rt http.RoundTripper, server io.Closer) {
	defer func() {
		err := server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	}()
	client := http.Client{
		Transport: rt,
	}
	defer client.CloseIdleConnections()

	resp, err := client.Get(url)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	fmt.Println(string(b))
}

func TestHTTPServer(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateHTTPServer(t)
	addresses := server.Addresses()

	t.Log("pprof http address:\n", addresses)
	t.Log("pprof http info:\n", server.Info())

	URL := fmt.Sprintf("http://admin:123456@%s/debug/pprof/", addresses[0])

	testFetch(t, URL, nil, server)
}

func TestHTTPSServer(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, tlsConfig := testGenerateHTTPSServer(t)
	addresses := server.Addresses()

	t.Log("pprof https address:\n", addresses)
	t.Log("pprof https info:\n", server.Info())

	URL := fmt.Sprintf("http://admin:123456@%s/debug/pprof/", addresses[0])
	transport := new(http.Transport)
	var err error
	transport.TLSClientConfig, err = tlsConfig.Apply()
	require.NoError(t, err)

	testFetch(t, URL, nil, server)
}
