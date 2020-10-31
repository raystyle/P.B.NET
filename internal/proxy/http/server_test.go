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
	testsuite.WaitProxyServerServe(t, server, 1)
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
	testsuite.WaitProxyServerServe(t, server, 2)
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
		mu        sync.Mutex
	)
	dialContext := func(ctx context.Context, network, address string) (net.Conn, error) {
		mu.Lock()
		secondary = true
		mu.Unlock()
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
	testsuite.WaitProxyServerServe(t, server, 1)
	address := server.Addresses()[0].String()

	// make client
	URL, err := url.Parse("http://" + address)
	require.NoError(t, err)
	transport := http.Transport{Proxy: http.ProxyURL(URL)}

	testsuite.ProxyServer(t, server, &transport)

	require.True(t, secondary)
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

	t.Run("invalid username", func(t *testing.T) {
		opts := Options{
			Username: "user:",
		}
		_, err := NewHTTPServer("username", nil, &opts)
		require.EqualError(t, err, "username can not include character \":\"")
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

	server, err := NewHTTPServer(testTag, logger.Test, nil)
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

func TestServer_ListenAndServe_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("part", func(t *testing.T) {
		server, err := NewHTTPServer(testTag, logger.Test, nil)
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
			server, err = NewHTTPServer(testTag, logger.Test, nil)
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
		server, err := NewHTTPServer(testTag, logger.Test, nil)
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
			server, err = NewHTTPServer(testTag, logger.Test, nil)
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
		server := testGenerateHTTPProxyServer(t)

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
			server = testGenerateHTTPProxyServer(t)
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
		server, err := NewHTTPServer(testTag, logger.Test, &opts)
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
			server, err = NewHTTPServer(testTag, logger.Test, &opts)
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
		server := testGenerateHTTPProxyServer(t)

		serve := func() {
			err := server.ListenAndServe("tcp", "127.0.0.1:0")
			require.NoError(t, err)
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
			server = testGenerateHTTPProxyServer(t)
		}
		serve := func() {
			err := server.ListenAndServe("tcp", "127.0.0.1:0")
			require.NoError(t, err)
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
		server, err := NewHTTPServer(testTag, logger.Test, nil)
		require.NoError(t, err)

		var listener net.Listener

		init := func() {
			listener, err = net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
		}
		serve := func() {
			err := server.Serve(listener)
			require.NoError(t, err)
		}
		req := func() {
			URL := &url.URL{
				Scheme: "http",
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
			server, err = NewHTTPServer(testTag, logger.Test, nil)
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
				Scheme: "http",
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
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("without close", func(t *testing.T) {
		t.Run("part", func(t *testing.T) {
			opts := Options{
				Username: "admin",
				Password: "123456",
			}
			server, err := NewHTTPServer(testTag, logger.Test, &opts)
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
					Scheme: "http",
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
				server, err = NewHTTPServer(testTag, logger.Test, &opts)
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
					Scheme: "http",
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
			server, err := NewHTTPServer(testTag, logger.Test, &opts)
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
					Scheme: "http",
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
				server, err = NewHTTPServer(testTag, logger.Test, &opts)
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
					Scheme: "http",
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
				las, las, serve, serve,
				addrs, info, close1, close1,
			}
			testsuite.RunParallel(10, init, cleanup, fns...)

			testsuite.IsDestroyed(t, server)
		})
	})
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
	testsuite.WaitProxyServerServe(t, server, 1)

	newReq := func() *http.Request {
		URL := fmt.Sprintf("http://localhost:%s/", testsuite.HTTPServerPort)
		req, err := http.NewRequest(http.MethodConnect, URL, nil)
		require.NoError(t, err)
		return req
	}

	t.Run("don't implemented http.Hijacker", func(t *testing.T) {
		w := httptest.NewRecorder()
		server.handler.ServeHTTP(w, newReq())
	})

	t.Run("close remote conn with error", func(t *testing.T) {
		opts := Options{DialContext: func(context.Context, string, string) (net.Conn, error) {
			return testsuite.NewMockConnWithCloseError(), nil
		}}
		server, err := NewHTTPServer(testTag, logger.Test, &opts)
		require.NoError(t, err)
		go func() {
			err := server.ListenAndServe(testNetwork, testAddress)
			require.NoError(t, err)
		}()
		testsuite.WaitProxyServerServe(t, server, 1)

		// mock income CONNECT request
		go func() {
			w := testsuite.NewMockResponseWriter()
			server.handler.ServeHTTP(w, newReq())
		}()
		time.Sleep(500 * time.Millisecond)

		err = server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("failed to hijack", func(t *testing.T) {
		w := testsuite.NewMockResponseWriterWithHijackError()
		server.handler.ServeHTTP(w, newReq())
	})

	t.Run("failed to response", func(t *testing.T) {
		w := testsuite.NewMockResponseWriterWithWriteError()
		server.handler.ServeHTTP(w, newReq())
	})

	t.Run("close hijacked conn with error", func(t *testing.T) {
		w := testsuite.NewMockResponseWriterWithCloseError()

		conn, _, err := w.(http.Hijacker).Hijack()
		require.NoError(t, err)
		err = conn.Close()
		testsuite.IsMockConnCloseError(t, err)

		server.handler.ServeHTTP(w, newReq())
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
		testsuite.WaitProxyServerServe(t, server, 1)

		go func() {
			w := testsuite.NewMockResponseWriter()
			server.handler.ServeHTTP(w, newReq())
		}()
		time.Sleep(500 * time.Millisecond)

		err = server.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, server)
	})

	t.Run("close hijacked conn with panic", func(t *testing.T) {
		go func() {
			w := testsuite.NewMockResponseWriterWithClosePanic()
			server.handler.ServeHTTP(w, newReq())
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

	server := testGenerateHTTPProxyServer(t)
	address := server.Addresses()[0].String()

	client := http.Client{Transport: new(http.Transport)}
	defer client.CloseIdleConnections()

	t.Run("only username", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		auth := base64.StdEncoding.EncodeToString([]byte("admin"))
		req.Header.Set("Proxy-Authorization", "Basic "+auth)

		resp, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusProxyAuthRequired, resp.StatusCode)

		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("invalid username or password", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		auth := base64.StdEncoding.EncodeToString([]byte("admin1:!23"))
		req.Header.Set("Proxy-Authorization", "Basic "+auth)

		resp, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusProxyAuthRequired, resp.StatusCode)

		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("invalid base64 data", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		req.Header.Set("Proxy-Authorization", "Basic foo")

		resp, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusProxyAuthRequired, resp.StatusCode)

		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("no authentication header", func(t *testing.T) {
		resp, err := client.Get("http://" + address)
		require.NoError(t, err)

		require.Equal(t, http.StatusProxyAuthRequired, resp.StatusCode)

		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	t.Run("unsupported authentication method", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address, nil)
		require.NoError(t, err)
		req.Header.Set("Proxy-Authorization", "method not-support")

		resp, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusProxyAuthRequired, resp.StatusCode)

		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		err = resp.Body.Close()
		require.NoError(t, err)
	})

	err := server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, server)
}
