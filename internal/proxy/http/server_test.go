package http

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/option"
	"project/internal/testsuite"
	"project/internal/testsuite/testtls"
)

const (
	testTag     = "test"
	testNetwork = "tcp"
	testAddress = "localhost:0"
)

func testGenerateHTTPProxyServer(t *testing.T) *Server {
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	server, err := NewHTTPServer(testTag, logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	return server
}

func testGenerateHTTPSProxyServer(t *testing.T) (*Server, option.TLSConfig) {
	serverCfg, clientCfg := testtls.OptionPair(t, "127.0.0.1")
	opts := Options{
		Username: "admin",
	}
	opts.Server.TLSConfig = serverCfg
	server, err := NewHTTPSServer(testTag, logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	return server, clientCfg
}

func TestHTTPProxyServer(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateHTTPProxyServer(t)
	addresses := server.Addresses()

	t.Log("http proxy address:\n", addresses)
	t.Log("http proxy info:\n", server.Info())

	// make client
	URL, err := url.Parse("http://admin:123456@" + addresses[0].String())
	require.NoError(t, err)
	transport := http.Transport{Proxy: http.ProxyURL(URL)}

	testsuite.ProxyServer(t, server, &transport)
}

func TestHTTPSProxyServer(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, tlsConfig := testGenerateHTTPSProxyServer(t)
	addresses := server.Addresses()

	t.Log("https proxy address:\n", addresses)
	t.Log("https proxy info:\n", server.Info())

	// make client
	proxyURL, err := url.Parse("https://admin@" + addresses[1].String())
	require.NoError(t, err)
	transport := http.Transport{Proxy: http.ProxyURL(proxyURL)}
	transport.TLSClientConfig, err = tlsConfig.Apply()
	require.NoError(t, err)

	testsuite.ProxyServer(t, server, &transport)
}

func TestHTTPProxyServerWithSecondaryProxy(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	var (
		secondary bool
		mutex     sync.Mutex
	)
	dialContext := func(ctx context.Context, network, address string) (net.Conn, error) {
		mutex.Lock()
		secondary = true
		mutex.Unlock()
		return new(net.Dialer).DialContext(ctx, network, address)
	}
	opts := Options{
		DialContext: dialContext,
	}
	server, err := NewHTTPServer(testTag, logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	address := server.Addresses()[0].String()

	// make client
	URL, err := url.Parse("http://" + address)
	require.NoError(t, err)
	transport := http.Transport{Proxy: http.ProxyURL(URL)}

	testsuite.ProxyServer(t, server, &transport)

	require.True(t, secondary)
}

func TestServer_Authenticate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateHTTPProxyServer(t)
	address := server.Addresses()[0].String()

	client := http.Client{}
	defer client.CloseIdleConnections()

	t.Run("no authenticate method", func(t *testing.T) {
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
		req.Header.Set("Proxy-Authorization", "method not-support")
		resp, err := client.Do(req)
		require.NoError(t, err)
		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("invalid username/password", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		userInfo := url.UserPassword("admin1", "123")
		req.Header.Set("Proxy-Authorization", "Basic "+userInfo.String())
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

func TestNewServer(t *testing.T) {
	t.Run("empty tag", func(t *testing.T) {
		_, err := NewHTTPServer("", nil, nil)
		require.EqualError(t, err, "empty tag")
	})

	t.Run("failed to apply server options", func(t *testing.T) {
		opts := Options{}
		opts.Server.TLSConfig.ClientCAs = []string{"foo"}
		_, err := NewHTTPServer("server", nil, &opts)
		require.Error(t, err)
	})

	t.Run("failed to apply transport options", func(t *testing.T) {
		opts := Options{}
		opts.Transport.TLSClientConfig.RootCAs = []string{"foo"}
		_, err := NewHTTPServer("transport", nil, &opts)
		require.Error(t, err)
	})
}

func TestServer_ListenAndServe(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewHTTPServer(testTag, logger.Test, nil)
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

	server, err := NewHTTPServer(testTag, logger.Test, nil)
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

	server, err := NewHTTPSServer(testTag, logger.Test, nil)
	require.NoError(t, err)

	err = server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
}

func TestHandler_ServeHTTP(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewHTTPServer(testTag, logger.Test, nil)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)

	URL := fmt.Sprintf("http://localhost:%s/", testsuite.HTTPServerPort)
	req, err := http.NewRequest(http.MethodConnect, URL, nil)
	require.NoError(t, err)

	t.Run("don't implemented http.Hijacker", func(t *testing.T) {
		w := httptest.NewRecorder()
		server.handler.ServeHTTP(w, req)
	})

	t.Run("failed to hijack", func(t *testing.T) {
		w := testsuite.NewMockResponseWriterWithHijackError()
		server.handler.ServeHTTP(w, req)
	})

	t.Run("failed to response", func(t *testing.T) {
		w := testsuite.NewMockResponseWriterWithWriteError()
		server.handler.ServeHTTP(w, req)
	})

	t.Run("close remote connection error", func(t *testing.T) {
		opts := Options{DialContext: func(context.Context, string, string) (net.Conn, error) {
			return testsuite.NewMockConnWithCloseError(), nil
		}}
		server, err := NewHTTPServer(testTag, logger.Test, &opts)
		require.NoError(t, err)
		go func() {
			err := server.ListenAndServe(testNetwork, testAddress)
			require.NoError(t, err)
		}()
		time.Sleep(250 * time.Millisecond)
		go func() {
			w := testsuite.NewMockResponseWriter()
			server.handler.ServeHTTP(w, req)
		}()
		time.Sleep(500 * time.Millisecond)

		err = server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("copy with panic", func(t *testing.T) {
		opts := Options{DialContext: func(context.Context, string, string) (net.Conn, error) {
			return testsuite.NewMockConnWithReadPanic(), nil
		}}
		server, err := NewHTTPServer(testTag, logger.Test, &opts)
		require.NoError(t, err)
		go func() {
			err := server.ListenAndServe(testNetwork, testAddress)
			require.NoError(t, err)
		}()
		time.Sleep(250 * time.Millisecond)
		go func() {
			w := testsuite.NewMockResponseWriter()
			server.handler.ServeHTTP(w, req)
		}()
		time.Sleep(500 * time.Millisecond)

		err = server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("close with panic", func(t *testing.T) {
		go func() {
			w := testsuite.NewMockResponseWriterWithClosePanic()
			server.handler.ServeHTTP(w, req)
		}()
		time.Sleep(500 * time.Millisecond)

		err = server.Close()
		require.NoError(t, err)
	})

	err = server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
}

func TestHandler_authenticate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	server, err := NewHTTPServer(testTag, logger.Test, &opts)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/", nil)
	require.NoError(t, err)
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin")) // without ":"
	req.Header.Set("Proxy-Authorization", auth)

	server.handler.authenticate(w, req)

	err = server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
}
