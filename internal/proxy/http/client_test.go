package http

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestHTTPProxyClient(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateHTTPProxyServer(t)
	address := server.Addresses()[0].String()
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err := NewHTTPClient("tcp", address, &opts)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestHTTPSProxyClient(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, tlsConfig := testGenerateHTTPSProxyServer(t)
	address := server.Addresses()[0].String()
	opts := Options{
		Username:  "admin",
		TLSConfig: tlsConfig,
	}
	client, err := NewHTTPSClient("tcp", address, &opts)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestHTTPProxyClientCancelConnect(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateHTTPProxyServer(t)
	address := server.Addresses()[0].String()
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err := NewHTTPClient("tcp", address, &opts)
	require.NoError(t, err)

	testsuite.ProxyClientCancelConnect(t, server, client)
}

func TestHTTPProxyClientWithoutPassword(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewHTTPServer("test", logger.Test, nil)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe("tcp", "localhost:0")
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	address := server.Addresses()[0].String()
	client, err := NewHTTPClient("tcp", address, nil)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestNewHTTPProxyClientWithIncorrectUserInfo(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateHTTPProxyServer(t)
	address := server.Addresses()[0].String()
	opts := Options{
		Username: "admin",
		Password: "123457",
	}
	client, err := NewHTTPClient("tcp", address, &opts)
	require.NoError(t, err)

	_, err = client.Dial("tcp", "localhost:0")
	require.Error(t, err)

	testsuite.IsDestroyed(t, client)
	require.NoError(t, server.Close())
	testsuite.IsDestroyed(t, server)
}

func TestHTTPProxyClientFailure(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("unknown network", func(t *testing.T) {
		_, err := NewHTTPClient("foo", "localhost:0", nil)
		require.Error(t, err)
	})

	t.Run("connect unreachable proxy server", func(t *testing.T) {
		client, err := NewHTTPClient("tcp", "localhost:0", nil)
		require.NoError(t, err)
		testsuite.ProxyClientWithUnreachableProxyServer(t, client)
	})

	t.Run("connect unreachable target", func(t *testing.T) {
		server := testGenerateHTTPProxyServer(t)
		address := server.Addresses()[0].String()
		opts := Options{
			Username: "admin",
			Password: "123456",
		}
		client, err := NewHTTPClient("tcp", address, &opts)
		require.NoError(t, err)

		testFailedToHandleHTTPRequest(t, client)
		testsuite.ProxyClientWithUnreachableTarget(t, server, client)
	})
}

func TestHTTPSProxyClientFailure(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("connect unreachable proxy server", func(t *testing.T) {
		client, err := NewHTTPSClient("tcp", "localhost:0", nil)
		require.NoError(t, err)
		testsuite.ProxyClientWithUnreachableProxyServer(t, client)
	})

	t.Run("connect unreachable target", func(t *testing.T) {
		server, tlsConfig := testGenerateHTTPSProxyServer(t)
		address := server.Addresses()[0].String()
		opts := Options{
			Username:  "admin",
			TLSConfig: tlsConfig,
		}
		client, err := NewHTTPSClient("tcp", address, &opts)
		require.NoError(t, err)

		testFailedToHandleHTTPRequest(t, client)
		testsuite.ProxyClientWithUnreachableTarget(t, server, client)
	})
}

func testFailedToHandleHTTPRequest(t testing.TB, client *Client) {
	transport := new(http.Transport)
	client.HTTP(transport)
	httpClient := http.Client{Transport: transport}
	resp, err := httpClient.Get("http://0.0.0.1/")
	require.NoError(t, err)
	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestFailedToNewClient(t *testing.T) {
	t.Run("unsupported network", func(t *testing.T) {
		_, err := NewHTTPClient("foo network", "", nil)
		require.Error(t, err)
	})

	t.Run("failed to apply tls config", func(t *testing.T) {
		opts := Options{}
		opts.TLSConfig.RootCAs = []string{"foo CA"}
		_, err := NewHTTPSClient("tcp", "", &opts)
		require.Error(t, err)
	})

	t.Run("invalid address", func(t *testing.T) {
		_, err := NewHTTPSClient("tcp", "", nil)
		require.Error(t, err)
	})
}

func TestHTTPSClientWithCertificate(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	serverCfg, clientCfg := testsuite.TLSConfigOptionPair(t)
	opts := Options{}
	opts.Server.TLSConfig = serverCfg
	server, err := NewHTTPSServer("test", logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe("tcp", "localhost:0")
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)

	address := server.Addresses()[0].String()
	opts = Options{TLSConfig: clientCfg}
	client, err := NewHTTPSClient("tcp", address, &opts)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestClient_Connect(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const network = "tcp"
	client, err := NewHTTPClient(network, "127.0.0.1:0", nil)
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("failed to write request", func(t *testing.T) {
		srv, cli := net.Pipe()
		defer func() { require.NoError(t, cli.Close()) }()
		require.NoError(t, srv.Close())
		_, err = client.Connect(ctx, cli, network, "127.0.0.1:1")
		require.Error(t, err)
	})

	t.Run("invalid response", func(t *testing.T) {
		srv, cli := net.Pipe()
		defer func() {
			require.NoError(t, srv.Close())
			require.NoError(t, cli.Close())
		}()
		wg := sync.WaitGroup{}
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(ioutil.Discard, srv)
		}()
		go func() {
			defer wg.Done()
			_, _ = srv.Write([]byte("HTTP/1.0 302 Connection established\r\n\r\n"))
		}()
		_, err = client.Connect(ctx, cli, network, "127.0.0.1:1")
		require.Error(t, err)
		require.NoError(t, cli.Close())
		wg.Wait()
	})
}
