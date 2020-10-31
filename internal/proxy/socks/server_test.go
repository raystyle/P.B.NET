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
	testsuite.WaitProxyServerServe(t, server, 1)
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
	testsuite.WaitProxyServerServe(t, server, 2)
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
	testsuite.WaitProxyServerServe(t, server, 1)
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
	testsuite.WaitProxyServerServe(t, server, 1)
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
		testsuite.WaitProxyServerServe(t, server, 1)

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
		server.counter.Done()

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

	const (
		tag     = "test"
		network = "tcp"
		address = "127.0.0.1:0"
	)

	listener, err := net.Listen(network, address)
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()
	addr := listener.Addr().String()

	infos := []string{
		"socks5",
		"socks5, auth: admin:",
		"socks5, address: [tcp " + addr + "], auth: admin:",
		"socks4a, user id: admin",
	}
	servers := make([]*Server, 0, len(infos))

	server, err := NewSocks5Server(tag, logger.Test, nil)
	require.NoError(t, err)
	servers = append(servers, server)

	server, err = NewSocks5Server(tag, logger.Test, &Options{Username: "admin"})
	require.NoError(t, err)
	servers = append(servers, server)

	serverA, err := NewSocks5Server(tag, logger.Test, &Options{Username: "admin"})
	require.NoError(t, err)
	go func() {
		err := serverA.Serve(listener)
		require.NoError(t, err)
	}()
	testsuite.WaitProxyServerServe(t, serverA, 1)
	servers = append(servers, serverA)

	server, err = NewSocks4aServer(tag, logger.Test, &Options{UserID: "admin"})
	require.NoError(t, err)
	servers = append(servers, server)

	for i := 0; i < len(infos); i++ {
		require.Equal(t, infos[i], servers[i].Info())
	}
}

func TestServer_ListenAndServe_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("part", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		las := func() {
			go func() {
				err := server.ListenAndServe("tcp", "127.0.0.1:0")
				require.NoError(t, err)
			}()
			time.Sleep(250 * time.Millisecond)
		}
		testsuite.RunParallel(10, nil, nil, las, las)

		err = server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("whole", func(t *testing.T) {
		var server *Server

		init := func() {
			var err error
			server, err = NewSocks5Server(testTag, logger.Test, nil)
			require.NoError(t, err)
		}
		las := func() {
			go func(server *Server) {
				err := server.ListenAndServe("tcp", "127.0.0.1:0")
				require.NoError(t, err)
			}(server)
			time.Sleep(250 * time.Millisecond)
		}
		cleanup := func() {
			err := server.Close()
			require.NoError(t, err)
		}
		testsuite.RunParallel(10, init, cleanup, las, las)

		testsuite.IsDestroyed(t, server)
	})
}

func TestServer_Serve_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("part", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		var (
			listener1 net.Listener
			listener2 net.Listener
		)

		init := func() {
			listener1, err = net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			listener2, err = net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
		}
		serve1 := func() {
			go func(listener net.Listener) {
				err := server.Serve(listener)
				require.NoError(t, err)
			}(listener1)
			time.Sleep(250 * time.Millisecond)
		}
		serve2 := func() {
			go func(listener net.Listener) {
				err := server.Serve(listener)
				require.NoError(t, err)
			}(listener2)
			time.Sleep(250 * time.Millisecond)
		}
		cleanup := func() {
			err := listener1.Close()
			require.NoError(t, err)
			err = listener2.Close()
			require.NoError(t, err)
		}
		testsuite.RunParallel(10, init, cleanup, serve1, serve2)

		err = server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("whole", func(t *testing.T) {
		var server *Server

		init := func() {
			var err error
			server, err = NewSocks5Server(testTag, logger.Test, nil)
			require.NoError(t, err)
		}
		serve := func() {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			go func(server *Server) {
				err := server.Serve(listener)
				require.NoError(t, err)
			}(server)
			time.Sleep(250 * time.Millisecond)
		}
		cleanup := func() {
			err := server.Close()
			require.NoError(t, err)
		}
		testsuite.RunParallel(10, init, cleanup, serve, serve)

		testsuite.IsDestroyed(t, server)
	})
}

func TestServer_Addresses_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("part", func(t *testing.T) {
		server := testGenerateSocks5Server(t)

		addrs := func() {
			addrs := server.Addresses()
			require.Len(t, addrs, 1)
		}
		testsuite.RunParallel(10, nil, nil, addrs, addrs)

		err := server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("whole", func(t *testing.T) {
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
	})
}

func TestServer_Info_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("part", func(t *testing.T) {
		opts := Options{
			Username: "admin",
			Password: "123456",
		}
		server, err := NewSocks5Server(testTag, logger.Test, &opts)
		require.NoError(t, err)

		var (
			listener1 net.Listener
			listener2 net.Listener
		)

		init := func() {
			listener1, err = net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			listener2, err = net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
		}
		serve1 := func() {
			go func(listener net.Listener) {
				err := server.Serve(listener)
				require.NoError(t, err)
			}(listener1)
			time.Sleep(250 * time.Millisecond)
		}
		serve2 := func() {
			go func(listener net.Listener) {
				err := server.Serve(listener)
				require.NoError(t, err)
			}(listener2)
			time.Sleep(250 * time.Millisecond)
		}
		info := func() {
			for i := 0; i < 3; i++ {
				t.Log(i, server.Info())
				time.Sleep(100 * time.Millisecond)
			}
		}
		cleanup := func() {
			err := listener1.Close()
			require.NoError(t, err)
			err = listener2.Close()
			require.NoError(t, err)
		}
		testsuite.RunParallel(10, init, cleanup, serve1, serve2, info)

		err = server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("whole", func(t *testing.T) {
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
			go func(server *Server) {
				err := server.ListenAndServe("tcp", "127.0.0.1:0")
				require.NoError(t, err)
			}(server)
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
	})
}

func TestServer_Close_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("part", func(t *testing.T) {
		server := testGenerateSocks5Server(t)

		serve := func() {
			_ = server.ListenAndServe("tcp", "127.0.0.1:0")
		}
		close1 := func() {
			err := server.Close()
			require.NoError(t, err)
		}
		testsuite.RunParallel(10, nil, nil, serve, serve, close1, close1)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("whole", func(t *testing.T) {
		var server *Server

		init := func() {
			server = testGenerateSocks5Server(t)
		}
		serve := func() {
			_ = server.ListenAndServe("tcp", "127.0.0.1:0")
		}
		close1 := func() {
			err := server.Close()
			require.NoError(t, err)
		}
		testsuite.RunParallel(10, init, nil, serve, serve, close1, close1)

		testsuite.IsDestroyed(t, server)
	})
}

func TestServer_NewRequest_Parallel(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("part", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		var listener net.Listener

		init := func() {
			listener, err = net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
		}
		serve := func() {
			_ = server.Serve(listener)
		}
		req := func() {
			URL := &url.URL{
				Scheme: "socks5",
				Host:   listener.Addr().String(),
				User:   url.UserPassword("admin", "123456"),
			}
			tr := http.Transport{
				Proxy: http.ProxyURL(URL),
			}
			client := http.Client{
				Transport: &tr,
				Timeout:   time.Second,
			}
			defer client.CloseIdleConnections()
			resp, err := client.Get("http://127.0.0.1:" + testsuite.HTTPServerPort)
			if err != nil {
				return
			}
			_, _ = ioutil.ReadAll(resp.Body)
		}
		close1 := func() {
			time.Sleep(250 * time.Millisecond)

			err := server.Close()
			require.NoError(t, err)
		}
		testsuite.RunParallel(10, init, nil, serve, req, req, close1, close1)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("whole", func(t *testing.T) {
		var (
			server   *Server
			listener net.Listener
		)

		init := func() {
			var err error
			server, err = NewSocks5Server(testTag, logger.Test, nil)
			require.NoError(t, err)
			listener, err = net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
		}
		serve := func() {
			err := server.Serve(listener)
			require.NoError(t, err)
		}
		req := func() {
			URL := &url.URL{
				Scheme: "socks5",
				Host:   listener.Addr().String(),
				User:   url.UserPassword("admin", "123456"),
			}
			tr := http.Transport{
				Proxy: http.ProxyURL(URL),
			}
			client := http.Client{
				Transport: &tr,
				Timeout:   time.Second,
			}
			defer client.CloseIdleConnections()
			resp, err := client.Get("http://127.0.0.1:" + testsuite.HTTPServerPort)
			if err != nil {
				return
			}
			_, _ = ioutil.ReadAll(resp.Body)
		}
		close1 := func() {
			time.Sleep(250 * time.Millisecond)

			err := server.Close()
			require.NoError(t, err)
		}
		testsuite.RunParallel(10, init, nil, serve, req, req, close1, close1)

		testsuite.IsDestroyed(t, server)
	})
}

func TestServer_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("without close", func(t *testing.T) {
		t.Run("part", func(t *testing.T) {
			opts := Options{
				Username: "admin",
				Password: "123456",
			}
			server, err := NewSocks5Server(testTag, logger.Test, &opts)
			require.NoError(t, err)

			var listener net.Listener

			init := func() {
				listener, err = net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
			}
			las := func() {
				go func() {
					err := server.ListenAndServe("tcp", "127.0.0.1:0")
					require.NoError(t, err)
				}()
			}
			serve := func() {
				go func(listener net.Listener) {
					err := server.Serve(listener)
					require.NoError(t, err)
				}(listener)
			}
			addrs := func() {
				for i := 0; i < 3; i++ {
					t.Log(i, server.Addresses())
					time.Sleep(100 * time.Millisecond)
				}
			}
			info := func() {
				for i := 0; i < 3; i++ {
					t.Log(i, server.Info())
					time.Sleep(100 * time.Millisecond)
				}
			}
			req := func() {
				URL := &url.URL{
					Scheme: "socks",
					Host:   listener.Addr().String(),
					User:   url.UserPassword("admin", "123456"),
				}
				tr := http.Transport{
					Proxy: http.ProxyURL(URL),
				}
				client := http.Client{
					Transport: &tr,
					Timeout:   time.Second,
				}
				defer client.CloseIdleConnections()
				resp, err := client.Get("http://127.0.0.1:" + testsuite.HTTPServerPort)
				if err != nil {
					return
				}
				_, _ = ioutil.ReadAll(resp.Body)
			}
			cleanup := func() {
				err := listener.Close()
				require.NoError(t, err)
			}
			testsuite.RunParallel(10, init, cleanup, las, serve, req, addrs, info)

			err = server.Close()
			require.NoError(t, err)

			testsuite.IsDestroyed(t, server)
		})

		t.Run("whole", func(t *testing.T) {
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
			las := func() {
				go func(server *Server) {
					err := server.ListenAndServe("tcp", "127.0.0.1:0")
					require.NoError(t, err)
				}(server)
			}
			serve := func() {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				go func(server *Server) {
					err := server.Serve(listener)
					require.NoError(t, err)
				}(server)

				// new request
				URL := &url.URL{
					Scheme: "socks",
					Host:   listener.Addr().String(),
					User:   url.UserPassword("admin", "123456"),
				}
				tr := http.Transport{
					Proxy: http.ProxyURL(URL),
				}
				client := http.Client{
					Transport: &tr,
					Timeout:   time.Second,
				}
				defer client.CloseIdleConnections()
				resp, err := client.Get("http://127.0.0.1:" + testsuite.HTTPServerPort)
				if err != nil {
					return
				}
				_, _ = ioutil.ReadAll(resp.Body)
			}
			addrs := func() {
				for i := 0; i < 3; i++ {
					t.Log(i, server.Addresses())
					time.Sleep(100 * time.Millisecond)
				}
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
			testsuite.RunParallel(10, init, cleanup, las, serve, addrs, info)

			testsuite.IsDestroyed(t, server)
		})
	})

	t.Run("with close", func(t *testing.T) {
		t.Run("part", func(t *testing.T) {
			opts := Options{
				Username: "admin",
				Password: "123456",
			}
			server, err := NewSocks5Server(testTag, logger.Test, &opts)
			require.NoError(t, err)

			var listener net.Listener

			init := func() {
				listener, err = net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
			}
			las := func() {
				go func() {
					_ = server.ListenAndServe("tcp", "127.0.0.1:0")
				}()
			}
			serve := func() {
				go func(listener net.Listener) {
					_ = server.Serve(listener)
				}(listener)
			}
			addrs := func() {
				for i := 0; i < 3; i++ {
					t.Log(i, server.Addresses())
					time.Sleep(100 * time.Millisecond)
				}
			}
			info := func() {
				for i := 0; i < 3; i++ {
					t.Log(i, server.Info())
					time.Sleep(100 * time.Millisecond)
				}
			}
			req := func() {
				URL := &url.URL{
					Scheme: "socks5",
					Host:   listener.Addr().String(),
					User:   url.UserPassword("admin", "123456"),
				}
				tr := http.Transport{
					Proxy: http.ProxyURL(URL),
				}
				client := http.Client{
					Transport: &tr,
					Timeout:   time.Second,
				}
				defer client.CloseIdleConnections()
				resp, err := client.Get("http://127.0.0.1:" + testsuite.HTTPServerPort)
				if err != nil {
					return
				}
				_, _ = ioutil.ReadAll(resp.Body)
			}
			close1 := func() {
				err := server.Close()
				require.NoError(t, err)
			}
			cleanup := func() {
				_ = listener.Close()
			}
			fns := []func(){
				las, las, serve, req, req,
				addrs, info, close1, close1,
			}
			testsuite.RunParallel(10, init, cleanup, fns...)

			err = server.Close()
			require.NoError(t, err)

			testsuite.IsDestroyed(t, server)
		})

		t.Run("whole", func(t *testing.T) {
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
			las := func() {
				go func(server *Server) {
					_ = server.ListenAndServe("tcp", "127.0.0.1:0")
				}(server)
			}
			serve := func() {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				go func(server *Server) {
					_ = server.Serve(listener)
				}(server)

				// new request
				URL := &url.URL{
					Scheme: "socks5",
					Host:   listener.Addr().String(),
					User:   url.UserPassword("admin", "123456"),
				}
				tr := http.Transport{
					Proxy: http.ProxyURL(URL),
				}
				client := http.Client{
					Transport: &tr,
					Timeout:   time.Second,
				}
				defer client.CloseIdleConnections()
				resp, err := client.Get("http://127.0.0.1:" + testsuite.HTTPServerPort)
				if err != nil {
					return
				}
				_, _ = ioutil.ReadAll(resp.Body)
			}
			addrs := func() {
				for i := 0; i < 3; i++ {
					t.Log(i, server.Addresses())
					time.Sleep(100 * time.Millisecond)
				}
			}
			info := func() {
				for i := 0; i < 3; i++ {
					t.Log(i, server.Info())
					time.Sleep(100 * time.Millisecond)
				}
			}
			close1 := func() {
				err := server.Close()
				require.NoError(t, err)
			}
			cleanup := func() {
				err := server.Close()
				require.NoError(t, err)
			}
			fns := []func(){
				las, las, serve, addrs, info,
				close1, close1,
			}
			testsuite.RunParallel(10, init, cleanup, fns...)

			testsuite.IsDestroyed(t, server)
		})
	})
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
			ctx:   server,
			local: testsuite.NewMockConn(),
		}
		conn.Serve()
		time.Sleep(250 * time.Millisecond)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("serve panic", func(t *testing.T) {
		server, err := NewSocks5Server(testTag, logger.Test, nil)
		require.NoError(t, err)

		conn := &conn{
			ctx:   server,
			local: testsuite.NewMockConnWithSetDeadlinePanic(),
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
