package http

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
	"project/internal/testsuite/testtls"
)

func TestNewClient(t *testing.T) {
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

	testsuite.ProxyClientWithHTTPSTarget(t, client)

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

	testsuite.ProxyClientWithHTTPSTarget(t, client)

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

	server, err := NewHTTPServer(testTag, logger.Test, nil)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	testsuite.WaitProxyServerServe(t, server, 1)
	address := server.Addresses()[0].String()
	client, err := NewHTTPClient("tcp", address, nil)
	require.NoError(t, err)

	testsuite.ProxyClient(t, server, client)
}

func TestNewHTTPProxyClientWithAuthenticate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateHTTPProxyServer(t)
	address := server.Addresses()[0].String()

	t.Run("invalid password", func(t *testing.T) {
		opts := Options{
			Username: "admin",
			Password: "123457",
		}
		client, err := NewHTTPClient("tcp", address, &opts)
		require.NoError(t, err)

		_, err = client.Dial("tcp", "localhost:0")
		require.Error(t, err)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("not write password", func(t *testing.T) {
		client, err := NewHTTPClient("tcp", address, nil)
		require.NoError(t, err)

		_, err = client.Dial("tcp", "localhost:0")
		require.Error(t, err)

		testsuite.IsDestroyed(t, client)
	})

	err := server.Close()
	require.NoError(t, err)
	testsuite.IsDestroyed(t, server)
}

func TestHTTPSClientWithCertificate(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	serverCfg, clientCfg := testtls.OptionPair(t, "127.0.0.1")
	opts := Options{}
	opts.Server.TLSConfig = serverCfg
	server, err := NewHTTPSServer(testTag, logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	testsuite.WaitProxyServerServe(t, server, 1)

	address := server.Addresses()[0].String()
	opts = Options{TLSConfig: clientCfg}
	client, err := NewHTTPSClient("tcp", address, &opts)
	require.NoError(t, err)

	testsuite.ProxyClientWithHTTPSTarget(t, client)

	testsuite.ProxyClient(t, server, client)
}

func TestHTTPProxyClientFailure(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

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

func TestClient_Connect(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const network = "tcp"
	client, err := NewHTTPClient(network, "127.0.0.1:0", nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("failed to write request", func(t *testing.T) {
		conn := testsuite.NewMockConnWithWriteError()

		_, err = client.Connect(ctx, conn, network, "127.0.0.1:1")
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("failed to read response", func(t *testing.T) {
		conn := testsuite.NewMockConnWithReadError()

		_, err = client.Connect(ctx, conn, network, "127.0.0.1:1")
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("invalid part response", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(cli net.Conn) {
				_, err = client.Connect(ctx, cli, network, "127.0.0.1:1")
				require.Error(t, err)
				t.Log(err)

				err = cli.Close()
				require.NoError(t, err)
			},
			func(srv net.Conn) {
				go func() { _, _ = io.Copy(ioutil.Discard, srv) }()
				_, _ = srv.Write([]byte("HTTP/1.0x200"))
			},
		)
	})

	t.Run("invalid status code", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(cli net.Conn) {
				_, err = client.Connect(ctx, cli, network, "127.0.0.1:1")
				require.Error(t, err)
				t.Log(err)

				err = cli.Close()
				require.NoError(t, err)
			},
			func(srv net.Conn) {
				go func() { _, _ = io.Copy(ioutil.Discard, srv) }()
				_, _ = srv.Write([]byte("HTTP/1.0 foo"))
			},
		)
	})

	t.Run("unexpected status code", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(cli net.Conn) {
				_, err = client.Connect(ctx, cli, network, "127.0.0.1:1")
				require.Error(t, err)
				t.Log(err)

				err = cli.Close()
				require.NoError(t, err)
			},
			func(srv net.Conn) {
				go func() { _, _ = io.Copy(ioutil.Discard, srv) }()
				_, _ = srv.Write([]byte("HTTP/1.0 302"))
			},
		)
	})

	t.Run("failed to read rest response", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(cli net.Conn) {
				_, err = client.Connect(ctx, cli, network, "127.0.0.1:1")
				require.Error(t, err)
				t.Log(err)

				err = cli.Close()
				require.NoError(t, err)
			},
			func(srv net.Conn) {
				go func() { _, _ = io.Copy(ioutil.Discard, srv) }()
				_, err := srv.Write([]byte("HTTP/1.0 200 Connection"))
				require.NoError(t, err)

				err = srv.Close()
				require.NoError(t, err)
			},
		)
	})

	t.Run("unexpected response", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(cli net.Conn) {
				_, err = client.Connect(ctx, cli, network, "127.0.0.1:1")
				require.Error(t, err)
				t.Log(err)

				err = cli.Close()
				require.NoError(t, err)
			},
			func(srv net.Conn) {
				go func() { _, _ = io.Copy(ioutil.Discard, srv) }()
				_, _ = srv.Write([]byte("HTTP/1.0 200 foo response\r\n\r\n"))
			},
		)
	})

	t.Run("context error", func(t *testing.T) {
		ctx, cancel := testsuite.NewMockContextWithError()
		defer cancel()
		conn := testsuite.NewMockConnWithWriteError()

		_, err = client.Connect(ctx, conn, network, "127.0.0.1:1")
		testsuite.IsMockContextError(t, err)
	})

	t.Run("panic from conn write", func(t *testing.T) {
		conn := testsuite.NewMockConnWithWritePanic()

		_, err = client.Connect(ctx, conn, network, "127.0.0.1:1")
		testsuite.IsMockConnWritePanic(t, err)
	})

	testsuite.IsDestroyed(t, client)
}
