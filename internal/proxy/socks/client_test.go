package socks

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestSocks5Client(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks5Server(t)
	address := server.Addresses()[0].String()
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err := NewSocks5Client("tcp", address, &opts)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestSocks4aClient(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks4aServer(t)
	address := server.Addresses()[0].String()
	opts := Options{
		UserID: "admin",
	}
	client, err := NewSocks4aClient("tcp", address, &opts)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestSocks4Client(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks4Server(t)
	address := server.Addresses()[0].String()
	opts := Options{
		UserID: "admin",
	}
	client, err := NewSocks4Client("tcp", address, &opts)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestSocks5ClientCancelConnect(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks5Server(t)
	address := server.Addresses()[0].String()
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	client, err := NewSocks5Client("tcp", address, &opts)
	require.NoError(t, err)

	testsuite.ProxyClientCancelConnect(t, server, client)
}

func TestSocks5ClientWithoutPassword(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewSocks5Server(testTag, logger.Test, nil)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	address := server.Addresses()[0].String()
	client, err := NewSocks5Client("tcp", address, nil)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestSocks4aClientWithoutUserID(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewSocks4aServer(testTag, logger.Test, nil)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	address := server.Addresses()[0].String()
	client, err := NewSocks4aClient("tcp", address, nil)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestSocks5Authenticate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks5Server(t)
	address := server.Addresses()[0].String()
	opt := Options{
		Username: "admin",
		Password: "123457",
	}
	client, err := NewSocks5Client("tcp", address, &opt)
	require.NoError(t, err)

	_, err = client.Dial("tcp", "localhost:0")
	require.Error(t, err)

	testsuite.IsDestroyed(t, client)

	err = server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
}

func TestSocks4aUserID(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks4aServer(t)
	address := server.Addresses()[0].String()
	opt := Options{
		UserID: "foo-user-id",
	}
	client, err := NewSocks4aClient("tcp", address, &opt)
	require.NoError(t, err)

	_, err = client.Dial("tcp", "localhost:0")
	require.Error(t, err)

	testsuite.IsDestroyed(t, client)

	err = server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
}

func TestSocks5ClientFailure(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("unknown network", func(t *testing.T) {
		_, err := NewSocks5Client("foo", "localhost:0", nil)
		require.Error(t, err)
	})

	t.Run("connect unreachable proxy server", func(t *testing.T) {
		client, err := NewSocks5Client("tcp", "localhost:0", nil)
		require.NoError(t, err)
		testsuite.ProxyClientWithUnreachableProxyServer(t, client)
	})

	t.Run("connect unreachable target", func(t *testing.T) {
		server := testGenerateSocks5Server(t)
		opts := Options{
			Username: "admin",
			Password: "123456",
		}
		address := server.Addresses()[0].String()
		client, err := NewSocks5Client("tcp", address, &opts)
		require.NoError(t, err)

		testsuite.ProxyClientWithUnreachableTarget(t, server, client)
	})
}

func TestSocks4aClientFailure(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("connect unreachable proxy server", func(t *testing.T) {
		client, err := NewSocks4aClient("tcp", "localhost:0", nil)
		require.NoError(t, err)
		testsuite.ProxyClientWithUnreachableProxyServer(t, client)
	})

	t.Run("connect unreachable target", func(t *testing.T) {
		server := testGenerateSocks4aServer(t)
		opts := Options{
			UserID: "admin",
		}
		address := server.Addresses()[0].String()
		client, err := NewSocks4aClient("tcp", address, &opts)
		require.NoError(t, err)

		testsuite.ProxyClientWithUnreachableTarget(t, server, client)
	})
}

func TestClient_Connect(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client, err := NewSocks5Client("tcp", "localhost:0", nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("foo address", func(t *testing.T) {
		_, err = client.Connect(ctx, nil, "tcp", "foo")
		require.Error(t, err)
	})

	t.Run("context error", func(t *testing.T) {
		srv, cli := net.Pipe()
		defer func() {
			err := srv.Close()
			require.NoError(t, err)
			err = cli.Close()
			require.NoError(t, err)
		}()

		buf := new(bytes.Buffer)
		patch := func(interface{}, byte) error {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(buf, "WriteByte", patch)
		defer pg.Unpatch()

		ctx, cancel := testsuite.NewMockContextWithError()
		defer cancel()

		_, err = client.Connect(ctx, cli, "tcp", "127.0.0.1:1")
		require.Error(t, err)
	})

	t.Run("panic from context", func(t *testing.T) {
		srv, cli := net.Pipe()
		defer func() {
			err := srv.Close()
			require.NoError(t, err)
			err = cli.Close()
			require.NoError(t, err)
		}()

		buf := new(bytes.Buffer)
		patch := func(interface{}, byte) error {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(buf, "WriteByte", patch)
		defer pg.Unpatch()

		_, err = client.Connect(ctx, cli, "tcp", "127.0.0.1:1")
		require.Error(t, err)
	})

	testsuite.IsDestroyed(t, client)
}
