package socks

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

const (
	testTag     = "test"
	testNetwork = "tcp"
	testAddress = "localhost:0"
)

func testGenerateSocks5Server(t *testing.T) *Server {
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	server, err := NewSocks5Server(testTag, logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	return server
}

func testGenerateSocks4aServer(t *testing.T) *Server {
	opts := Options{
		UserID: "admin",
	}
	server, err := NewSocks4aServer(testTag, logger.Test, &opts)
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
	return server
}

func testGenerateSocks4Server(t *testing.T) *Server {
	opts := Options{
		UserID: "admin",
	}
	server, err := NewSocks4Server(testTag, logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	return server
}

func TestSocks5Server(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks5Server(t)
	addresses := server.Addresses()

	t.Log("socks5 address:", addresses)
	t.Log("socks5 info:", server.Info())

	// make client
	URL, err := url.Parse("socks5://admin:123456@" + addresses[0].String())
	require.NoError(t, err)
	transport := http.Transport{Proxy: http.ProxyURL(URL)}

	testsuite.ProxyServer(t, server, &transport)
}

func TestSocks4aServer(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks4aServer(t)
	server.userID = nil
	t.Log("socks4a address:", server.Addresses())
	t.Log("socks4a info:", server.Info())

	// use external tool to test it, because the http.Client
	// only support socks5, http and https
	// time.Sleep(30 * time.Second)

	err := server.Close()
	require.NoError(t, err)
	err = server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
}

func TestSocks4Server(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks4Server(t)
	server.userID = nil
	t.Log("socks4 address:", server.Addresses()[0])
	t.Log("socks4 info:", server.Info())

	// use external tool to test it, because the http.Client
	// only support socks5, http and https
	// time.Sleep(30 * time.Second)

	err := server.Close()
	require.NoError(t, err)
	err = server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
}

func TestSocks5ServerWithSecondaryProxy(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	var secondary atomic.Value
	secondary.Store(false)
	dialContext := func(ctx context.Context, network, address string) (net.Conn, error) {
		secondary.Store(true)
		return new(net.Dialer).DialContext(ctx, network, address)
	}
	opts := Options{
		DialContext: dialContext,
	}
	server, err := NewSocks5Server(testTag, logger.Test, &opts)
	require.NoError(t, err)
	go func() {
		err := server.ListenAndServe(testNetwork, testAddress)
		require.NoError(t, err)
	}()
	time.Sleep(250 * time.Millisecond)
	address := server.Addresses()[0].String()

	// make client
	URL, err := url.Parse("socks5://" + address)
	require.NoError(t, err)
	transport := http.Transport{Proxy: http.ProxyURL(URL)}

	testsuite.ProxyServer(t, server, &transport)

	require.True(t, secondary.Load().(bool))
}

func TestNewServerWithEmptyTag(t *testing.T) {
	_, err := NewSocks5Server("", nil, nil)
	require.EqualError(t, err, "empty tag")
}

func TestServer_ListenAndServe(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("failed", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		// invalid network
		err = server.ListenAndServe("foo", "localhost:0")
		require.Error(t, err)

		// invalid address
		err = server.ListenAndServe("tcp", "foo")
		require.Error(t, err)

		err = server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("shutting down", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		err = server.Close()
		require.NoError(t, err)

		err = server.ListenAndServe("foo", "foo")
		require.Equal(t, ErrServerClosed, err)

		testsuite.IsDestroyed(t, server)
	})
}

func TestServer_Serve(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("accept error", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		listener := testsuite.NewMockListenerWithAcceptError()
		err = server.Serve(listener)
		testsuite.IsMockListenerAcceptFatal(t, err)

		err = server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("accept panic", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		listener := testsuite.NewMockListenerWithAcceptPanic()
		err = server.Serve(listener)
		testsuite.IsMockListenerAcceptPanic(t, err)

		err = server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("close listener error", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		listener := testsuite.NewMockListenerWithCloseError()
		go func() {
			err := server.Serve(listener)
			testsuite.IsMockListenerClosedError(t, err)
		}()
		time.Sleep(250 * time.Millisecond)

		err = server.Close()
		testsuite.IsMockListenerCloseError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("shutting down", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		err = server.Close()
		require.NoError(t, err)

		listener := testsuite.NewMockListenerWithAcceptError()
		err = server.Serve(listener)
		require.Equal(t, ErrServerClosed, err)

		testsuite.IsDestroyed(t, server)
	})
}

func TestServer_Close(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("ok", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		err = server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("error about close listener", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		listener := testsuite.NewMockListenerWithCloseError()
		server.trackListener(&listener, true)

		err = server.Close()
		testsuite.IsMockListenerCloseError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("error about close connection", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		conn := &conn{local: testsuite.NewMockConnWithCloseError()}
		server.trackConn(conn, true)

		err = server.Close()
		testsuite.IsMockConnCloseError(t, err)

		testsuite.IsDestroyed(t, server)
	})
}

func TestServer_Info(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server := testGenerateSocks4Server(t)
	t.Log("socks4 info:", server.Info())

	err := server.Close()
	require.NoError(t, err)
	err = server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
}

func TestServer_ListenAndServe_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	var server *Server

	init := func() {
		var err error
		server, err = NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)
	}
	las := func() {
		go func() {
			err := server.ListenAndServe("tcp", "127.0.0.1:0")
			require.NoError(t, err)
		}()
		time.Sleep(250 * time.Millisecond)
	}
	cleanup := func() {
		err := server.Close()
		require.NoError(t, err)
	}
	testsuite.RunParallel(10, init, cleanup, las, las)

	testsuite.IsDestroyed(t, server)
}

func TestServer_Serve_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	var server *Server

	init := func() {
		var err error
		server, err = NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)
	}
	serve := func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		go func() {
			err := server.Serve(listener)
			require.NoError(t, err)
		}()
		time.Sleep(250 * time.Millisecond)
	}
	cleanup := func() {
		err := server.Close()
		require.NoError(t, err)
	}
	testsuite.RunParallel(10, init, cleanup, serve, serve)

	testsuite.IsDestroyed(t, server)
}

func TestServer_Addresses_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	var server *Server

	init := func() {
		server = testGenerateSocks5Server(t)
	}
	addrs := func() {
		addrs := server.Addresses()
		require.Len(t, addrs, 1)
	}
	cleanup := func() {
		err := server.Close()
		require.NoError(t, err)
	}
	testsuite.RunParallel(10, init, cleanup, addrs, addrs)

	testsuite.IsDestroyed(t, server)
}

func TestServer_Info_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	var server *Server

	init := func() {
		opts := Options{
			Username: "admin",
			Password: "123456",
		}
		var err error
		server, err = NewSocks5Server(testTag, logger.Test, &opts)
		require.NoError(t, err)
	}
	serve := func() {
		go func() {
			err := server.ListenAndServe("tcp", "127.0.0.1:0")
			require.NoError(t, err)
		}()
	}
	info := func() {
		for i := 0; i < 3; i++ {
			t.Log(i, server.Info())
			time.Sleep(100 * time.Millisecond)
		}
	}
	cleanup := func() {
		err := server.Close()
		require.NoError(t, err)
	}
	testsuite.RunParallel(10, init, cleanup, serve, serve, info)

	testsuite.IsDestroyed(t, server)
}

func TestConn_Serve(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("failed to track", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		err = server.Close()
		require.NoError(t, err)

		conn := &conn{
			server: server,
			local:  testsuite.NewMockConn(),
		}
		conn.Serve()
		time.Sleep(250 * time.Millisecond)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("serve panic", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		conn := &conn{
			server: server,
			local:  testsuite.NewMockConnWithSetDeadlinePanic(),
		}
		conn.Serve()
		time.Sleep(250 * time.Millisecond)

		err = server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("remote close", func(t *testing.T) {
		server := testGenerateSocks5Server(t)
		addresses := server.Addresses()

		// make http client
		URL, err := url.Parse("socks5://admin:123456@" + addresses[0].String())
		require.NoError(t, err)
		transport := http.Transport{Proxy: http.ProxyURL(URL)}
		client := http.Client{Transport: &transport}
		defer client.CloseIdleConnections()

		// patch
		conn := net.Conn(new(net.TCPConn))
		patch := func(c *net.TCPConn) error {
			_ = c.Close()
			return monkey.Error
		}
		pg := monkey.PatchInstanceMethod(conn, "Close", patch)
		defer pg.Unpatch()

		resp, err := client.Get("http://localhost:" + testsuite.HTTPServerPort)
		require.NoError(t, err)
		_, err = ioutil.ReadAll(resp.Body)
		require.NoError(t, err)

		err = server.Close()
		require.Error(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("copy panic", func(t *testing.T) {
		server := testGenerateSocks5Server(t)
		addresses := server.Addresses()

		// make http client
		URL, err := url.Parse("socks5://admin:123456@" + addresses[0].String())
		require.NoError(t, err)
		transport := http.Transport{Proxy: http.ProxyURL(URL)}
		client := http.Client{Transport: &transport}
		defer client.CloseIdleConnections()

		patch := func(io.Writer, io.Reader) (int64, error) {
			panic(monkey.Panic)
		}
		pg := monkey.Patch(io.Copy, patch)
		defer pg.Unpatch()

		_, err = client.Get("http://localhost:" + testsuite.HTTPServerPort)
		require.Error(t, err)

		err = server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})
}
