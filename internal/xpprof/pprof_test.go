package xpprof

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
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

	require.Equal(t, http.StatusOK, resp.StatusCode)

	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	fmt.Println(string(b))

	err = resp.Body.Close()
	require.NoError(t, err)
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

	URL := fmt.Sprintf("https://admin@%s/debug/pprof/", addresses[0])
	transport := new(http.Transport)
	var err error
	transport.TLSClientConfig, err = tlsConfig.Apply()
	require.NoError(t, err)

	testFetch(t, URL, transport, server)
}

func TestHandler_authenticate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateHTTPServer(t)
	address := server.Addresses()[0].String()

	client := http.Client{}
	defer client.CloseIdleConnections()

	t.Run("only username", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		userInfo := url.User("admin")
		auth := base64.StdEncoding.EncodeToString([]byte(userInfo.String()))
		req.Header.Set("Authorization", "Basic "+auth)

		resp, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("invalid username/password", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		userInfo := url.UserPassword("admin1", "123")
		auth := base64.StdEncoding.EncodeToString([]byte(userInfo.String()))
		req.Header.Set("Authorization", "Basic "+auth)

		resp, err := client.Do(req)
		require.NoError(t, err)

		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("invalid basic base64 data", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Basic foo")

		resp, err := client.Do(req)
		require.NoError(t, err)

		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("no authenticate header", func(t *testing.T) {
		resp, err := client.Get("http://" + address)
		require.NoError(t, err)
		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("unsupported authenticate method", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "method not-support")

		resp, err := client.Do(req)
		require.NoError(t, err)

		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	err := server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
}
