package xpprof

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"project/internal/logger"
	"project/internal/option"
	"project/internal/patch/monkey"
	"project/internal/patch/toml"
	"project/internal/testsuite"
	"project/internal/testsuite/testtls"
)

const (
	testNetwork = "tcp"
	testAddress = "localhost:0"
)

func testGenerateHTTPServer(t *testing.T) *Server {
	password, err := bcrypt.GenerateFromPassword([]byte("123456"), 12)
	require.NoError(t, err)
	opts := Options{
		Username: "admin",
		Password: string(password),
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

	client := http.Client{Transport: rt}
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

func TestHTTPServerWithoutPassword(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewHTTPServer(logger.Test, nil)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	testsuite.WaitProxyServerServe(t, server, 1)

	addresses := server.Addresses()

	t.Log("pprof http address:\n", addresses)
	t.Log("pprof http info:\n", server.Info())

	URL := fmt.Sprintf("http://%s/debug/pprof/", addresses[0])

	testFetch(t, URL, nil, server)
}

func TestNewServer(t *testing.T) {
	t.Run("invalid server options", func(t *testing.T) {
		opts := Options{}
		opts.Server.TLSConfig.ClientCAs = []string{"foo"}
		_, err := NewHTTPServer(nil, &opts)
		require.Error(t, err)
	})

	t.Run("invalid username", func(t *testing.T) {
		opts := Options{
			Username: "user:",
		}
		_, err := NewHTTPServer(nil, &opts)
		require.EqualError(t, err, "username can not include character \":\"")
	})

	t.Run("invalid password", func(t *testing.T) {
		opts := Options{
			Password: "foo bcrypt hash",
		}
		_, err := NewHTTPServer(nil, &opts)
		require.EqualError(t, err, "invalid bcrypt hash about password")
	})
}

func TestServer_ListenAndServe(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewHTTPServer(logger.Test, nil)
	require.NoError(t, err)

	err = server.ListenAndServe("foo", "localhost:0")
	require.Error(t, err)
	err = server.ListenAndServe("tcp", "foo")
	require.Error(t, err)

	err = server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
}

func TestServer_Serve(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewHTTPServer(logger.Test, nil)
	require.NoError(t, err)

	listener := testsuite.NewMockListenerWithAcceptError()
	err = server.Serve(listener)
	testsuite.IsMockListenerAcceptFatal(t, err)

	listener = testsuite.NewMockListenerWithAcceptPanic()
	err = server.Serve(listener)
	testsuite.IsMockListenerAcceptPanic(t, err)

	err = server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
}

func TestServer_Close(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewHTTPServer(logger.Test, nil)
	require.NoError(t, err)

	listener := testsuite.NewMockListenerWithCloseError()
	go func() {
		err := server.Serve(listener)
		require.NoError(t, err)
	}()
	testsuite.WaitProxyServerServe(t, server, 1)
	// wait http server Serve
	time.Sleep(time.Second)

	err = server.Close()
	require.Error(t, err)

	testsuite.IsDestroyed(t, server)
}

func TestHandler_ServeHTTP(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	patch := func(string, string) []string {
		panic(monkey.Panic)
	}
	pg := monkey.Patch(strings.Split, patch)
	defer pg.Unpatch()

	server := testGenerateHTTPServer(t)
	addresses := server.Addresses()

	URL := fmt.Sprintf("http://admin:123456@%s/debug/pprof/", addresses[0])

	testFetch(t, URL, nil, server)
}

func TestHandler_authenticate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateHTTPServer(t)
	address := server.Addresses()[0].String()

	client := http.Client{Transport: new(http.Transport)}
	defer client.CloseIdleConnections()

	t.Run("only username", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		auth := base64.StdEncoding.EncodeToString([]byte("admin"))
		req.Header.Set("Authorization", "Basic "+auth)

		resp, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("invalid username or password", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		auth := base64.StdEncoding.EncodeToString([]byte("admin1:123"))
		req.Header.Set("Authorization", "Basic "+auth)

		resp, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

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

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("no authentication header", func(t *testing.T) {
		resp, err := client.Get("http://" + address)
		require.NoError(t, err)

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("unsupported authentication method", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "method not-support")

		resp, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	err := server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
}

func TestOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/options.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := Options{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, opts)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "admin", actual: opts.Username},
		{expected: "bcrypt", actual: opts.Password},
		{expected: time.Minute, actual: opts.Timeout},
		{expected: 1000, actual: opts.MaxConns},
		{expected: 30 * time.Second, actual: opts.Server.ReadTimeout},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}
