package http

import (
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
		TLSConfig: *tlsConfig,
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
	defer func() {
		require.NoError(t, server.Close())
		testsuite.IsDestroyed(t, server)
	}()
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
			TLSConfig: *tlsConfig,
		}
		client, err := NewHTTPSClient("tcp", address, &opts)
		require.NoError(t, err)
		testsuite.ProxyClientWithUnreachableTarget(t, server, client)
	})
}
