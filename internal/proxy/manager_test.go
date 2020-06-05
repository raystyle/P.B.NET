package proxy

import (
	"io/ioutil"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
	"project/internal/testsuite/testcert"
)

var testServerTags = [...]string{
	"socks5",
	"socks4a",
	"socks4",
	"http",
	"https",
}

var testServerNum = len(testServerTags)

func testGenerateManager(t *testing.T) *Manager {
	pool := testcert.CertPool(t)
	manager := NewManager(pool, logger.Test, nil)
	for i, filename := range []string{
		"socks/testdata/socks5_client.toml",
		"socks/testdata/socks4a_client.toml",
		"socks/testdata/socks4_client.toml",
		"http/testdata/http_client.toml",
		"http/testdata/https_client.toml",
	} {
		opts, err := ioutil.ReadFile(filename)
		require.NoError(t, err)
		server := &Server{
			Tag:     testServerTags[i],
			Mode:    testServerTags[i],
			Options: string(opts),
		}
		err = manager.Add(server)
		require.NoError(t, err)
	}
	return manager
}

func TestManager_Add(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := testGenerateManager(t)

	t.Run("with empty tag", func(t *testing.T) {
		err := manager.add(new(Server))
		require.EqualError(t, err, "empty proxy server tag")
	})

	t.Run("with unknown mode", func(t *testing.T) {
		err := manager.add(&Server{
			Tag:  "foo",
			Mode: "foo mode",
		})
		require.EqualError(t, err, "unknown mode: foo mode")
	})

	t.Run("exist", func(t *testing.T) {
		opts, err := ioutil.ReadFile("socks/testdata/socks5_server.toml")
		require.NoError(t, err)
		server := &Server{
			Tag:     ModeSocks5,
			Mode:    ModeSocks5,
			Options: string(opts),
		}
		const errStr = "failed to add proxy server socks5: already exists"
		err = manager.Add(server)
		require.EqualError(t, err, errStr)
	})

	t.Run("socks server with invalid toml data", func(t *testing.T) {
		err := manager.Add(&Server{
			Tag:     "invalid socks5",
			Mode:    ModeSocks5,
			Options: "socks4 = foo",
		})
		require.Error(t, err)
	})

	t.Run("http proxy server with invalid toml data", func(t *testing.T) {
		err := manager.Add(&Server{
			Tag:     "invalid http",
			Mode:    ModeHTTP,
			Options: "https = foo",
		})
		require.Error(t, err)
	})

	t.Run("http proxy server with invalid options", func(t *testing.T) {
		err := manager.Add(&Server{
			Tag:  "http with invalid options",
			Mode: ModeHTTP,
			Options: `
[transport]
  [transport.tls_config]
    root_ca = ["foo data"]
`,
		})
		require.Error(t, err)
	})

	servers := manager.Servers()
	require.Len(t, servers, testServerNum)

	t.Run("add after close", func(t *testing.T) {
		err := manager.Close()
		require.NoError(t, err)
		err = manager.Add(&Server{
			Tag:  ModeSocks5,
			Mode: ModeSocks5,
		})
		require.Error(t, err)
	})

	err := manager.Close()
	require.NoError(t, err)

	servers = manager.Servers()
	require.Empty(t, servers)

	testsuite.IsDestroyed(t, manager)
}

func TestManager_Get(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := testGenerateManager(t)

	t.Run("basic", func(t *testing.T) {
		for _, tag := range testServerTags {
			server, err := manager.Get(tag)
			require.NoError(t, err)
			require.NotNil(t, server)
		}
	})

	t.Run("empty tag", func(t *testing.T) {
		ps, err := manager.Get("")
		require.EqualError(t, err, "empty proxy server tag")
		require.Nil(t, ps)
	})

	t.Run("doesn't exist", func(t *testing.T) {
		ps, err := manager.Get("foo")
		require.EqualError(t, err, "proxy server foo doesn't exist")
		require.Nil(t, ps)
	})

	t.Run("print all", func(t *testing.T) {
		for tag, server := range manager.Servers() {
			const format = "tag: %s mode: %s info: %s\n"
			t.Logf(format, tag, server.Mode, server.Info())
		}
	})

	servers := manager.Servers()
	require.Len(t, servers, testServerNum)

	err := manager.Close()
	require.NoError(t, err)

	servers = manager.Servers()
	require.Empty(t, servers)

	testsuite.IsDestroyed(t, manager)
}

func TestManager_Delete(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := testGenerateManager(t)

	t.Run("basic", func(t *testing.T) {
		for _, tag := range testServerTags {
			err := manager.Delete(tag)
			require.NoError(t, err)
		}
		servers := manager.Servers()
		require.Empty(t, servers)
	})

	t.Run("empty tag", func(t *testing.T) {
		err := manager.Delete("")
		require.EqualError(t, err, "empty proxy server tag")
	})

	t.Run("doesn't exist", func(t *testing.T) {
		err := manager.Delete("foo")
		require.EqualError(t, err, "proxy server foo doesn't exist")
	})

	err := manager.Close()
	require.NoError(t, err)

	servers := manager.Servers()
	require.Empty(t, servers)

	testsuite.IsDestroyed(t, manager)
}

func TestManager_Close(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	pool := testcert.CertPool(t)
	manager := NewManager(pool, logger.Test, nil)
	server := Server{
		Tag:  "test",
		Mode: ModeSocks5,
	}
	err := manager.Add(&server)
	require.NoError(t, err)

	// patch
	var (
		listener *net.TCPListener
		pg       *monkey.PatchGuard
	)
	patch := func(listener *net.TCPListener) error {
		pg.Unpatch()
		err := listener.Close()
		require.NoError(t, err)
		return monkey.Error
	}
	pg = monkey.PatchInstanceMethod(listener, "Close", patch)
	defer pg.Unpatch()

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := server.ListenAndServe("tcp", "localhost:0")
		require.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		err = server.Serve(listener)
		require.NoError(t, err)
	}()
	// wait serve
	time.Sleep(250 * time.Millisecond)

	t.Log("create at:", server.CreateAt())
	t.Log("serve at:", server.ServeAt())

	err = manager.Close()
	monkey.IsMonkeyError(t, err)

	wg.Wait()

	servers := manager.Servers()
	require.Empty(t, servers)

	testsuite.IsDestroyed(t, manager)
}

func TestManager_Add_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test1"
		tag2 = "test2"
	)

	pool := testcert.CertPool(t)
	server1 := &Server{
		Tag:  tag1,
		Mode: ModeSocks5,
	}
	server2 := &Server{
		Tag:  tag2,
		Mode: ModeHTTP,
	}

	t.Run("part", func(t *testing.T) {
		manager := NewManager(pool, logger.Test, nil)

		add1 := func() {
			err := manager.Add(server1)
			require.NoError(t, err)
		}
		add2 := func() {
			err := manager.Add(server2)
			require.NoError(t, err)
		}
		cleanup := func() {
			servers := manager.Servers()
			require.Len(t, servers, 2)

			err := manager.Delete(tag1)
			require.NoError(t, err)
			err = manager.Delete(tag2)
			require.NoError(t, err)

			servers = manager.Servers()
			require.Empty(t, servers)
		}
		testsuite.RunParallel(100, nil, cleanup, add1, add2)

		err := manager.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, manager)
	})

	t.Run("whole", func(t *testing.T) {
		var manager *Manager

		init := func() {
			manager = NewManager(pool, logger.Test, nil)
		}
		add1 := func() {
			err := manager.Add(server1)
			require.NoError(t, err)
		}
		add2 := func() {
			err := manager.Add(server2)
			require.NoError(t, err)
		}
		cleanup := func() {
			err := manager.Close()
			require.NoError(t, err)

			servers := manager.Servers()
			require.Empty(t, servers)
		}
		testsuite.RunParallel(100, init, cleanup, add1, add2)

		testsuite.IsDestroyed(t, manager)
	})

	testsuite.IsDestroyed(t, pool)
	testsuite.IsDestroyed(t, server1)
	testsuite.IsDestroyed(t, server2)
}

func TestManager_Delete_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test1"
		tag2 = "test2"
	)

	pool := testcert.CertPool(t)
	server1 := &Server{
		Tag:  tag1,
		Mode: ModeSocks5,
	}
	server2 := &Server{
		Tag:  tag2,
		Mode: ModeHTTP,
	}

	t.Run("part", func(t *testing.T) {
		manager := NewManager(pool, logger.Test, nil)

		init := func() {
			err := manager.Add(server1)
			require.NoError(t, err)
			err = manager.Add(server2)
			require.NoError(t, err)
		}
		delete1 := func() {
			err := manager.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := manager.Delete(tag2)
			require.NoError(t, err)
		}
		cleanup := func() {
			servers := manager.Servers()
			require.Empty(t, servers)
		}
		testsuite.RunParallel(100, init, cleanup, delete1, delete2)

		err := manager.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, manager)
	})

	t.Run("whole", func(t *testing.T) {
		var manager *Manager

		init := func() {
			manager = NewManager(pool, logger.Test, nil)

			err := manager.Add(server1)
			require.NoError(t, err)
			err = manager.Add(server2)
			require.NoError(t, err)
		}

		delete1 := func() {
			err := manager.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := manager.Delete(tag2)
			require.NoError(t, err)
		}
		cleanup := func() {
			servers := manager.Servers()
			require.Empty(t, servers)

			err := manager.Close()
			require.NoError(t, err)
		}
		testsuite.RunParallel(100, init, cleanup, delete1, delete2)

		testsuite.IsDestroyed(t, manager)
	})

	testsuite.IsDestroyed(t, pool)
	testsuite.IsDestroyed(t, server1)
	testsuite.IsDestroyed(t, server2)
}

func TestManager_Get_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test1"
		tag2 = "test2"
	)

	pool := testcert.CertPool(t)
	server1 := &Server{
		Tag:  tag1,
		Mode: ModeSocks5,
	}
	server2 := &Server{
		Tag:  tag2,
		Mode: ModeHTTP,
	}

	t.Run("part", func(t *testing.T) {
		manager := NewManager(pool, logger.Test, nil)

		err := manager.Add(server1)
		require.NoError(t, err)
		err = manager.Add(server2)
		require.NoError(t, err)

		get1 := func() {
			server, err := manager.Get(tag1)
			require.NoError(t, err)
			require.NotNil(t, server)
			require.Equal(t, tag1, server.Tag)
		}
		get2 := func() {
			server, err := manager.Get(tag2)
			require.NoError(t, err)
			require.NotNil(t, server)
			require.Equal(t, tag2, server.Tag)
		}
		testsuite.RunParallel(100, nil, nil, get1, get2)

		servers := manager.Servers()
		require.Len(t, servers, 2)

		err = manager.Close()
		require.NoError(t, err)

		servers = manager.Servers()
		require.Empty(t, servers)

		testsuite.IsDestroyed(t, manager)
	})

	t.Run("whole", func(t *testing.T) {
		var manager *Manager

		init := func() {
			manager = NewManager(pool, logger.Test, nil)

			err := manager.Add(server1)
			require.NoError(t, err)
			err = manager.Add(server2)
			require.NoError(t, err)
		}
		get1 := func() {
			server, err := manager.Get(tag1)
			require.NoError(t, err)
			require.NotNil(t, server)
			require.Equal(t, tag1, server.Tag)
		}
		get2 := func() {
			server, err := manager.Get(tag2)
			require.NoError(t, err)
			require.NotNil(t, server)
			require.Equal(t, tag2, server.Tag)
		}
		cleanup := func() {
			servers := manager.Servers()
			require.Len(t, servers, 2)

			err := manager.Delete(tag1)
			require.NoError(t, err)
			err = manager.Delete(tag2)
			require.NoError(t, err)

			servers = manager.Servers()
			require.Empty(t, servers)
		}
		testsuite.RunParallel(100, init, cleanup, get1, get2)

		testsuite.IsDestroyed(t, manager)
	})

	testsuite.IsDestroyed(t, pool)
	testsuite.IsDestroyed(t, server1)
	testsuite.IsDestroyed(t, server2)
}

func TestManager_Servers_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test1"
		tag2 = "test2"
	)

	pool := testcert.CertPool(t)
	server1 := &Server{
		Tag:  tag1,
		Mode: ModeSocks5,
	}
	server2 := &Server{
		Tag:  tag2,
		Mode: ModeHTTP,
	}

	t.Run("part", func(t *testing.T) {
		manager := NewManager(pool, logger.Test, nil)

		err := manager.Add(server1)
		require.NoError(t, err)
		err = manager.Add(server2)
		require.NoError(t, err)

		servers := func() {
			servers := manager.Servers()
			require.Len(t, servers, 2)
		}
		testsuite.RunParallel(100, nil, nil, servers, servers)

		err = manager.Close()
		require.NoError(t, err)

		s := manager.Servers()
		require.Empty(t, s)

		testsuite.IsDestroyed(t, manager)
	})

	t.Run("whole", func(t *testing.T) {
		var manager *Manager

		init := func() {
			manager = NewManager(pool, logger.Test, nil)

			err := manager.Add(server1)
			require.NoError(t, err)
			err = manager.Add(server2)
			require.NoError(t, err)
		}
		servers := func() {
			servers := manager.Servers()
			require.Len(t, servers, 2)
		}
		cleanup := func() {
			err := manager.Delete(tag1)
			require.NoError(t, err)
			err = manager.Delete(tag2)
			require.NoError(t, err)

			servers := manager.Servers()
			require.Empty(t, servers)
		}
		testsuite.RunParallel(100, init, cleanup, servers, servers)

		testsuite.IsDestroyed(t, manager)
	})

	testsuite.IsDestroyed(t, pool)
	testsuite.IsDestroyed(t, server1)
	testsuite.IsDestroyed(t, server2)
}

func TestManager_Close_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test1"
		tag2 = "test2"
	)

	pool := testcert.CertPool(t)
	server1 := &Server{
		Tag:  tag1,
		Mode: ModeSocks5,
	}
	server2 := &Server{
		Tag:  tag2,
		Mode: ModeHTTP,
	}

	t.Run("part", func(t *testing.T) {
		manager := NewManager(pool, logger.Test, nil)

		err := manager.Add(server1)
		require.NoError(t, err)
		err = manager.Add(server2)
		require.NoError(t, err)

		close1 := func() {
			err := manager.Close()
			require.NoError(t, err)
		}
		testsuite.RunParallel(100, nil, nil, close1, close1)

		servers := manager.Servers()
		require.Empty(t, servers)

		testsuite.IsDestroyed(t, manager)
	})

	t.Run("whole", func(t *testing.T) {
		var manager *Manager

		init := func() {
			manager = NewManager(pool, logger.Test, nil)

			err := manager.Add(server1)
			require.NoError(t, err)
			err = manager.Add(server2)
			require.NoError(t, err)
		}
		close1 := func() {
			err := manager.Close()
			require.NoError(t, err)
		}
		testsuite.RunParallel(100, init, nil, close1, close1)

		testsuite.IsDestroyed(t, manager)
	})

	testsuite.IsDestroyed(t, pool)
	testsuite.IsDestroyed(t, server1)
	testsuite.IsDestroyed(t, server2)
}

func TestManager_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test1" // for init and delete
		tag2 = "test2" // for init and delete
		tag3 = "test3" // for add
		tag4 = "test4" // for add
		tag5 = "test5" // for get
		tag6 = "test6" // for get
	)

	pool := testcert.CertPool(t)
	server1 := &Server{
		Tag:  tag1,
		Mode: ModeSocks5,
	}
	server2 := &Server{
		Tag:  tag2,
		Mode: ModeHTTP,
	}
	server3 := &Server{
		Tag:  tag3,
		Mode: ModeSocks4a,
	}
	server4 := &Server{
		Tag:  tag4,
		Mode: ModeHTTPS,
	}
	server5 := &Server{
		Tag:  tag5,
		Mode: ModeSocks4,
	}
	server6 := &Server{
		Tag:  tag6,
		Mode: ModeHTTPS,
	}

	t.Run("part", func(t *testing.T) {
		t.Run("without close", func(t *testing.T) {
			manager := NewManager(pool, logger.Test, nil)

			init := func() {
				err := manager.Add(server1)
				require.NoError(t, err)
				err = manager.Add(server2)
				require.NoError(t, err)
				err = manager.Add(server5)
				require.NoError(t, err)
				err = manager.Add(server6)
				require.NoError(t, err)
			}
			add1 := func() {
				err := manager.Add(server3)
				require.NoError(t, err)
			}
			add2 := func() {
				err := manager.Add(server4)
				require.NoError(t, err)
			}
			delete1 := func() {
				err := manager.Delete(tag1)
				require.NoError(t, err)
			}
			delete2 := func() {
				err := manager.Delete(tag2)
				require.NoError(t, err)
			}
			get1 := func() {
				server, err := manager.Get(tag5)
				require.NoError(t, err)
				require.NotNil(t, server)
				require.Equal(t, tag5, server.Tag)
			}
			get2 := func() {
				server, err := manager.Get(tag6)
				require.NoError(t, err)
				require.NotNil(t, server)
				require.Equal(t, tag6, server.Tag)
			}
			servers := func() {
				servers := manager.Servers()
				require.NotEmpty(t, servers)
			}
			cleanup := func() {
				err := manager.Delete(tag3)
				require.NoError(t, err)
				err = manager.Delete(tag4)
				require.NoError(t, err)
				err = manager.Delete(tag5)
				require.NoError(t, err)
				err = manager.Delete(tag6)
				require.NoError(t, err)

				servers := manager.Servers()
				require.Empty(t, servers)
			}
			fns := []func(){
				add1, add2, delete1, delete2,
				get1, get2, servers, servers,
			}
			testsuite.RunParallel(100, init, cleanup, fns...)

			err := manager.Close()
			require.NoError(t, err)

			testsuite.IsDestroyed(t, manager)
		})

		t.Run("with close", func(t *testing.T) {
			manager := NewManager(pool, logger.Test, nil)

			init := func() {
				_ = manager.Add(server1)
				_ = manager.Add(server2)
				_ = manager.Add(server5)
				_ = manager.Add(server6)
			}
			add1 := func() {
				_ = manager.Add(server3)
			}
			add2 := func() {
				_ = manager.Add(server4)
			}
			delete1 := func() {
				_ = manager.Delete(tag1)
			}
			delete2 := func() {
				_ = manager.Delete(tag2)
			}
			get1 := func() {
				_, _ = manager.Get(tag5)
			}
			get2 := func() {
				_, _ = manager.Get(tag6)
			}
			servers := func() {
				_ = manager.Servers()
			}
			close1 := func() {
				err := manager.Close()
				require.NoError(t, err)
			}
			fns := []func(){
				add1, add2, delete1, delete2,
				get1, get2, servers, servers,
				close1,
			}
			testsuite.RunParallel(100, init, nil, fns...)

			testsuite.IsDestroyed(t, manager)
		})
	})

	t.Run("whole", func(t *testing.T) {
		t.Run("without close", func(t *testing.T) {
			var manager *Manager

			init := func() {
				manager = NewManager(pool, logger.Test, nil)

				err := manager.Add(server1)
				require.NoError(t, err)
				err = manager.Add(server2)
				require.NoError(t, err)
				err = manager.Add(server5)
				require.NoError(t, err)
				err = manager.Add(server6)
				require.NoError(t, err)
			}
			add1 := func() {
				err := manager.Add(server3)
				require.NoError(t, err)
			}
			add2 := func() {
				err := manager.Add(server4)
				require.NoError(t, err)
			}
			delete1 := func() {
				err := manager.Delete(tag1)
				require.NoError(t, err)
			}
			delete2 := func() {
				err := manager.Delete(tag2)
				require.NoError(t, err)
			}
			get1 := func() {
				server, err := manager.Get(tag5)
				require.NoError(t, err)
				require.NotNil(t, server)
				require.Equal(t, tag5, server.Tag)
			}
			get2 := func() {
				server, err := manager.Get(tag6)
				require.NoError(t, err)
				require.NotNil(t, server)
				require.Equal(t, tag6, server.Tag)
			}
			servers := func() {
				servers := manager.Servers()
				require.NotEmpty(t, servers)
			}
			cleanup := func() {
				err := manager.Close()
				require.NoError(t, err)

				servers := manager.Servers()
				require.Empty(t, servers)
			}
			fns := []func(){
				add1, add2, delete1, delete2,
				get1, get2, servers, servers,
			}
			testsuite.RunParallel(100, init, cleanup, fns...)

			testsuite.IsDestroyed(t, manager)
		})

		t.Run("with close", func(t *testing.T) {
			var manager *Manager

			init := func() {
				manager = NewManager(pool, logger.Test, nil)

				err := manager.Add(server1)
				require.NoError(t, err)
				err = manager.Add(server2)
				require.NoError(t, err)
				err = manager.Add(server5)
				require.NoError(t, err)
				err = manager.Add(server6)
				require.NoError(t, err)
			}
			add1 := func() {
				_ = manager.Add(server3)
			}
			add2 := func() {
				_ = manager.Add(server4)
			}
			delete1 := func() {
				_ = manager.Delete(tag1)
			}
			delete2 := func() {
				_ = manager.Delete(tag2)
			}
			get1 := func() {
				_, _ = manager.Get(tag5)
			}
			get2 := func() {
				_, _ = manager.Get(tag6)
			}
			servers := func() {
				_ = manager.Servers()
			}
			close1 := func() {
				err := manager.Close()
				require.NoError(t, err)
			}
			fns := []func(){
				add1, add2, delete1, delete2,
				get1, get2, servers, servers,
				close1,
			}
			testsuite.RunParallel(100, init, nil, fns...)

			testsuite.IsDestroyed(t, manager)
		})
	})

	testsuite.IsDestroyed(t, pool)
	testsuite.IsDestroyed(t, server1)
	testsuite.IsDestroyed(t, server2)
	testsuite.IsDestroyed(t, server3)
	testsuite.IsDestroyed(t, server4)
	testsuite.IsDestroyed(t, server5)
	testsuite.IsDestroyed(t, server6)
}
