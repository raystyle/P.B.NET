package dns

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/nettool"
	"project/internal/patch/monkey"
	"project/internal/patch/toml"
	"project/internal/random"
	"project/internal/testsuite"
	"project/internal/testsuite/testproxy"
)

func TestClient_Add(t *testing.T) {
	client := NewClient(nil, nil)

	t.Run("ok", func(t *testing.T) {
		err := client.Add("ok", &Server{
			Method:  MethodUDP,
			Address: "1.1.1.1:53",
		})
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		err := client.Add("", nil)
		require.Error(t, err)
	})

	t.Run("empty tag", func(t *testing.T) {
		err := client.add("", nil)
		require.EqualError(t, err, "empty tag")
	})

	t.Run("empty method", func(t *testing.T) {
		err := client.add("foo", new(Server))
		require.EqualError(t, err, "empty method")
	})

	t.Run("empty address", func(t *testing.T) {
		err := client.add("foo", &Server{Method: MethodUDP})
		require.EqualError(t, err, "empty address")
	})

	t.Run("unknown method", func(t *testing.T) {
		err := client.add("foo", &Server{
			Method:  "foo method",
			Address: "1.1.1.1",
		})
		require.EqualError(t, err, "unknown method: foo method")
	})

	t.Run("exist", func(t *testing.T) {
		const tag = "two"

		server := &Server{
			Method:  MethodUDP,
			Address: "1.1.1.1:53",
		}
		err := client.add(tag, server)
		require.NoError(t, err)
		err = client.add(tag, server)
		require.EqualError(t, err, "is already exists")
	})

	testsuite.IsDestroyed(t, client)
}

func TestClient_Delete(t *testing.T) {
	client := NewClient(nil, nil)

	t.Run("ok", func(t *testing.T) {
		const tag = "test"

		server := &Server{
			Method:  MethodUDP,
			Address: "1.1.1.1:53",
		}
		err := client.Add(tag, server)
		require.NoError(t, err)

		err = client.Delete(tag)
		require.NoError(t, err)
	})

	t.Run("is not exist", func(t *testing.T) {
		err := client.Delete("foo tag")
		require.Error(t, err)
		t.Log(err)
	})

	testsuite.IsDestroyed(t, client)
}

func testAddAllDNSServers(t *testing.T, client *Client) {
	if testsuite.IPv4Enabled {
		testAddDNSServers(t, client, "dns_ipv4.toml")
	}
	if testsuite.IPv6Enabled {
		testAddDNSServers(t, client, "dns_ipv6.toml")
	}
	// ds: double stack
	testAddDNSServers(t, client, "dns_ds.toml")
}

func testAddDNSServers(t *testing.T, client *Client, filename string) {
	servers := make(map[string]*Server)
	data, err := ioutil.ReadFile("testdata/" + filename)
	require.NoError(t, err)
	err = toml.Unmarshal(data, &servers)
	require.NoError(t, err)
	for tag, server := range servers {
		err = client.Add(tag, server)
		require.NoError(t, err)
	}
}

func TestClient_Resolve(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	proxyPool, proxyMgr, certPool := testproxy.PoolAndManager(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	client := NewClient(certPool, proxyPool)
	testAddAllDNSServers(t, client)

	t.Run("print dns servers", func(t *testing.T) {
		for tag, server := range client.Servers() {
			t.Log(tag, server.Address)
		}
	})

	t.Run("use default options", func(t *testing.T) {
		result, err := client.Resolve(testDomain, nil)
		require.NoError(t, err)
		require.NotEmpty(t, result)
		t.Log("use default options", result)
	})

	t.Run("use method DoH", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{Method: MethodDoH}
		result, err := client.Resolve(testDomain, opts)
		require.NoError(t, err)
		require.NotEmpty(t, result)
		t.Log("use DoH:", result)
	})

	t.Run("resolve type IPv6", func(t *testing.T) {
		client.FlushCache()

		result, err := client.Resolve(testDomain, &Options{Type: TypeIPv6})
		require.NoError(t, err)
		require.NotEmpty(t, result)
		t.Log("resolve IPv6:", result)
	})

	t.Run("resolve punycode", func(t *testing.T) {
		client.FlushCache()

		result, err := client.Resolve("错的是.世界", nil)
		require.NoError(t, err)
		require.NotEmpty(t, result)
		t.Log("resolve punycode:", result)
	})

	t.Run("use system mode", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{Mode: ModeSystem}

		if testsuite.IPv4Enabled {
			t.Run("IPv4", func(t *testing.T) {
				opts.Type = TypeIPv4
				result, err := client.Resolve(testDomain, opts)
				require.NoError(t, err)
				require.NotEmpty(t, result)
				t.Log("IPv4:", result)
			})
		}

		if testsuite.IPv6Enabled {
			t.Run("IPv6", func(t *testing.T) {
				opts.Type = TypeIPv6
				result, err := client.Resolve(testDomain, opts)
				require.NoError(t, err)
				require.NotEmpty(t, result)
				t.Log("IPv6:", result)
			})
		}

		if testsuite.IPv4Enabled || testsuite.IPv6Enabled {
			t.Run("default", func(t *testing.T) {
				opts.Type = ""
				result, err := client.Resolve(testDomain, opts)
				require.NoError(t, err)
				require.NotEmpty(t, result)
				t.Log("IPv4 & IPv6:", result)
			})
		}

		t.Run("invalid type", func(t *testing.T) {
			opts.Type = "foo type"
			result, err := client.Resolve(testDomain, opts)
			require.Error(t, err)
			require.Empty(t, result)
		})
	})

	t.Run("resolve IP address", func(t *testing.T) {
		client.FlushCache()

		result, err := client.Resolve("1.1.1.1", nil)
		require.NoError(t, err)
		require.Equal(t, []string{"1.1.1.1"}, result)

		result, err = client.Resolve("::1", nil)
		require.NoError(t, err)
		require.Equal(t, []string{"::1"}, result)
	})

	t.Run("empty domain", func(t *testing.T) {
		client.FlushCache()

		result, err := client.Resolve("", nil)
		require.Error(t, err)
		require.Empty(t, result)
	})

	testsuite.IsDestroyed(t, client)
}

func TestClient_selectType(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	proxyPool, proxyMgr, certPool := testproxy.PoolAndManager(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	client := NewClient(certPool, proxyPool)
	testAddAllDNSServers(t, client)

	ipv4Enabled, ipv6Enabled := nettool.IPEnabled()
	const domain = "one.one.one.one"
	ctx := context.Background()
	opts := &Options{Mode: ModeSystem}

	t.Run("IPv4 Only", func(t *testing.T) {
		if !ipv4Enabled {
			return
		}

		patch := func() (bool, bool) {
			return true, false
		}
		pg := monkey.Patch(nettool.IPEnabled, patch)
		defer pg.Unpatch()

		ip, err := client.selectType(ctx, domain, opts)
		require.NoError(t, err)
		require.Contains(t, ip, "1.1.1.1")
	})

	t.Run("IPv6 Only", func(t *testing.T) {
		if !ipv6Enabled {
			return
		}

		patch := func() (bool, bool) {
			return false, true
		}
		pg := monkey.Patch(nettool.IPEnabled, patch)
		defer pg.Unpatch()

		ip, err := client.selectType(ctx, domain, opts)
		require.NoError(t, err)
		require.Contains(t, ip, "2606:4700:4700::1111")
	})

	t.Run("network unavailable", func(t *testing.T) {
		patch := func() (bool, bool) {
			return false, false
		}
		pg := monkey.Patch(nettool.IPEnabled, patch)
		defer pg.Unpatch()

		ip, err := client.selectType(ctx, domain, opts)
		require.Error(t, err)
		require.Nil(t, ip)
	})

	testsuite.IsDestroyed(t, client)
}

func TestClient_Cache(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	proxyPool, proxyMgr, certPool := testproxy.PoolAndManager(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	client := NewClient(certPool, proxyPool)
	testAddAllDNSServers(t, client)

	result, err := client.Resolve(testDomain, nil)
	require.NoError(t, err)
	require.NotEmpty(t, result)
	t.Log("[no cache]:", result)

	result, err = client.Resolve(testDomain, nil)
	require.NoError(t, err)
	require.NotEmpty(t, result)
	t.Log("[cache]:", result)

	testsuite.IsDestroyed(t, client)
}

func TestClient_Cancel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	proxyPool, proxyMgr, certPool := testproxy.PoolAndManager(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	client := NewClient(certPool, proxyPool)
	testAddAllDNSServers(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	opts := &Options{Method: MethodTCP}
	result, err := client.ResolveContext(ctx, testDomain, opts)
	require.Error(t, err)
	require.Empty(t, result)

	testsuite.IsDestroyed(t, client)
}

func TestClient_NoResult(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	proxyPool, proxyMgr, certPool := testproxy.PoolAndManager(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	client := NewClient(certPool, proxyPool)

	if testsuite.IPv4Enabled {
		err := client.Add("reachable-ipv4", &Server{
			Method:  MethodUDP,
			Address: "1.1.1.1:53",
		})
		require.NoError(t, err)
	}

	if testsuite.IPv6Enabled {
		err := client.Add("reachable-ipv6", &Server{
			Method:  MethodUDP,
			Address: "[2606:4700:4700::1111]:53",
		})
		require.NoError(t, err)
	}

	opts := &Options{Method: MethodUDP}
	result, err := client.Resolve("test", opts)
	require.Error(t, err)
	require.Empty(t, result)

	testsuite.IsDestroyed(t, client)
}

func TestOptions_Clone(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	proxyPool, proxyMgr, certPool := testproxy.PoolAndManager(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	client := NewClient(certPool, proxyPool)
	testAddAllDNSServers(t, client)
	client.DisableCache()

	const domain = "cloudflare.com"

	dotOpts1 := &Options{
		Method: MethodDoT,
	}
	dotOpts2 := &Options{
		Method: MethodDoT,
	}
	dohOpts1 := &Options{
		Method: MethodDoT,
	}
	dohOpts2 := &Options{
		Method: MethodDoT,
	}
	dot := func() {
		_, err := client.Resolve(domain, dotOpts1)
		require.NoError(t, err)
	}
	doh := func() {
		_, err := client.Resolve(domain, dohOpts1)
		require.NoError(t, err)
	}
	testsuite.RunMultiTimes(1, dot, dot, doh, doh)

	require.Equal(t, dotOpts2, dotOpts1)
	require.Equal(t, dohOpts2, dohOpts1)

	testsuite.IsDestroyed(t, client)
}

func TestClient_TestServers(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	proxyPool, proxyMgr, certPool := testproxy.PoolAndManager(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
	ctx := context.Background()

	t.Run("reachable and skip test", func(t *testing.T) {
		client := NewClient(certPool, proxyPool)

		opts := new(Options)

		// no DNS server
		result, err := client.TestServers(ctx, testDomain, opts)
		require.Equal(t, err, ErrNoDNSServers)
		require.Empty(t, result)

		// add reachable and skip test
		if testsuite.IPv4Enabled {
			err := client.Add("reachable-ipv4", &Server{
				Method:  MethodUDP,
				Address: "1.1.1.1:53",
			})
			require.NoError(t, err)
		}
		if testsuite.IPv6Enabled {
			err := client.Add("reachable-ipv6", &Server{
				Method:  MethodUDP,
				Address: "[2606:4700:4700::1111]:53",
			})
			require.NoError(t, err)
		}
		err = client.Add("skip_test", &Server{
			Method:   MethodUDP,
			Address:  "1.1.1.1:53",
			SkipTest: true,
		})
		require.NoError(t, err)

		result, err = client.TestServers(ctx, testDomain, opts)
		require.NoError(t, err)
		require.NotEmpty(t, result)
		t.Log(result)

		if testsuite.IPv4Enabled {
			err := client.Delete("reachable-ipv4")
			require.NoError(t, err)
		}
		if testsuite.IPv6Enabled {
			err := client.Delete("reachable-ipv6")
			require.NoError(t, err)
		}

		testsuite.IsDestroyed(t, client)
	})

	t.Run("unreachable DNS server", func(t *testing.T) {
		client := NewClient(certPool, proxyPool)

		err := client.Add("unreachable", &Server{
			Method:  MethodUDP,
			Address: "1.2.3.4",
		})
		require.NoError(t, err)
		result, err := client.TestServers(ctx, testDomain, new(Options))
		require.Error(t, err)
		require.Empty(t, result)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("cancel", func(t *testing.T) {
		client := NewClient(certPool, proxyPool)
		testAddAllDNSServers(t, client)

		ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()
		result, err := client.TestServers(ctx, testDomain, new(Options))
		require.Error(t, err)
		require.Empty(t, result)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("panic", func(t *testing.T) {
		client := NewClient(certPool, proxyPool)
		testAddAllDNSServers(t, client)

		opts := new(Options)
		patch := func(interface{}) *Options {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(opts, "Clone", patch)
		defer pg.Unpatch()

		result, err := client.TestServers(ctx, testDomain, new(Options))
		require.Error(t, err)
		require.Empty(t, result)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_TestOption(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	proxyPool, proxyMgr, certPool := testproxy.PoolAndManager(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
	client := NewClient(certPool, proxyPool)
	testAddAllDNSServers(t, client)

	ctx := context.Background()

	t.Run("skip test", func(t *testing.T) {
		opts := &Options{SkipTest: true}
		result, err := client.TestOption(ctx, testDomain, opts)
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("skip proxy", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{
			ProxyTag:  "tag",
			SkipProxy: true,
		}
		result, err := client.TestOption(ctx, testDomain, opts)
		require.NoError(t, err)
		require.NotEmpty(t, result)
	})

	t.Run("invalid domain name", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{Mode: ModeSystem}
		result, err := client.TestOption(ctx, "test", opts)
		require.Error(t, err)
		require.Empty(t, result)
	})

	t.Run("with proxy tag", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{
			Method:   MethodTCP, // must don't use udp
			ProxyTag: testproxy.TagBalance,
		}
		result, err := client.TestOption(ctx, testDomain, opts)
		require.NoError(t, err)
		require.NotEmpty(t, result)
	})

	t.Run("unknown type", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{Type: "foo type"}
		result, err := client.TestOption(ctx, testDomain, opts)
		require.Error(t, err)
		require.Empty(t, result)
		t.Log("test option:", err)
	})

	t.Run("unknown mode", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{Mode: "foo mode"}
		result, err := client.TestOption(ctx, testDomain, opts)
		require.Error(t, err)
		require.Empty(t, result)
	})

	t.Run("unknown method", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{Method: "foo method"}
		result, err := client.TestOption(ctx, testDomain, opts)
		require.Error(t, err)
		require.Empty(t, result)
	})

	t.Run("invalid http transport options", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{Method: MethodDoH}
		opts.Transport.TLSClientConfig.RootCAs = []string{"foo ca"}
		result, err := client.TestOption(ctx, testDomain, opts)
		require.Error(t, err)
		require.Empty(t, result)

		// with server tag
		opts.ServerTag = "doh_ipv4_cloudflare"
		result, err = client.TestOption(ctx, testDomain, opts)
		require.Error(t, err)
		require.Empty(t, result)
	})

	t.Run("doesn't exist proxy", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{ProxyTag: "foo proxy"}
		result, err := client.TestOption(ctx, testDomain, opts)
		require.Error(t, err)
		require.Empty(t, result)
	})

	t.Run("doesn't exist server tag", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{ServerTag: "foo server"}
		result, err := client.TestOption(ctx, testDomain, opts)
		require.Error(t, err)
		require.Empty(t, result)
	})

	testsuite.IsDestroyed(t, client)
}

func TestClient_Add_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test-01"
		tag2 = "test-02"
	)

	server1 := &Server{
		Method:  MethodUDP,
		Address: "127.0.0.1:53",
	}
	server2 := &Server{
		Method:  MethodDoT,
		Address: "127.0.0.1:853",
	}

	t.Run("part", func(t *testing.T) {
		client := NewClient(nil, nil)

		add1 := func() {
			err := client.Add(tag1, server1)
			require.NoError(t, err)
		}
		add2 := func() {
			err := client.Add(tag2, server2)
			require.NoError(t, err)
		}
		cleanup := func() {
			servers := client.Servers()
			require.Len(t, servers, 2)

			err := client.Delete(tag1)
			require.NoError(t, err)
			err = client.Delete(tag2)
			require.NoError(t, err)

			servers = client.Servers()
			require.Empty(t, servers)
		}
		testsuite.RunParallel(100, nil, cleanup, add1, add2)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("whole", func(t *testing.T) {
		var client *Client

		init := func() {
			client = NewClient(nil, nil)
		}
		add1 := func() {
			err := client.Add(tag1, server1)
			require.NoError(t, err)
		}
		add2 := func() {
			err := client.Add(tag2, server2)
			require.NoError(t, err)
		}
		cleanup := func() {
			servers := client.Servers()
			require.Len(t, servers, 2)

			err := client.Delete(tag1)
			require.NoError(t, err)
			err = client.Delete(tag2)
			require.NoError(t, err)

			servers = client.Servers()
			require.Empty(t, servers)
		}
		testsuite.RunParallel(100, init, cleanup, add1, add2)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_Delete_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test-01"
		tag2 = "test-02"
	)

	server1 := &Server{
		Method:  MethodUDP,
		Address: "127.0.0.1:53",
	}
	server2 := &Server{
		Method:  MethodDoT,
		Address: "127.0.0.1:853",
	}

	t.Run("part", func(t *testing.T) {
		client := NewClient(nil, nil)

		init := func() {
			err := client.Add(tag1, server1)
			require.NoError(t, err)
			err = client.Add(tag2, server2)
			require.NoError(t, err)
		}
		delete1 := func() {
			err := client.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := client.Delete(tag2)
			require.NoError(t, err)
		}
		cleanup := func() {
			servers := client.Servers()
			require.Empty(t, servers)
		}
		testsuite.RunParallel(100, init, cleanup, delete1, delete2)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("whole", func(t *testing.T) {
		var client *Client

		init := func() {
			client = NewClient(nil, nil)

			err := client.Add(tag1, server1)
			require.NoError(t, err)
			err = client.Add(tag2, server2)
			require.NoError(t, err)
		}
		delete1 := func() {
			err := client.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := client.Delete(tag2)
			require.NoError(t, err)
		}
		cleanup := func() {
			servers := client.Servers()
			require.Empty(t, servers)
		}
		testsuite.RunParallel(100, init, cleanup, delete1, delete2)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_Servers_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test-01"
		tag2 = "test-02"
	)

	server1 := &Server{
		Method:  MethodUDP,
		Address: "127.0.0.1:53",
	}
	server2 := &Server{
		Method:  MethodDoT,
		Address: "127.0.0.1:853",
	}

	t.Run("part", func(t *testing.T) {
		client := NewClient(nil, nil)

		err := client.Add(tag1, server1)
		require.NoError(t, err)
		err = client.Add(tag2, server2)
		require.NoError(t, err)

		servers := func() {
			servers := client.Servers()
			require.Len(t, servers, 2)
		}
		testsuite.RunParallel(100, nil, nil, servers, servers)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("whole", func(t *testing.T) {
		var client *Client

		init := func() {
			client = NewClient(nil, nil)

			err := client.Add(tag1, server1)
			require.NoError(t, err)
			err = client.Add(tag2, server2)
			require.NoError(t, err)
		}
		servers := func() {
			servers := client.Servers()
			require.Len(t, servers, 2)
		}
		cleanup := func() {
			err := client.Delete(tag1)
			require.NoError(t, err)
			err = client.Delete(tag2)
			require.NoError(t, err)

			servers := client.Servers()
			require.Empty(t, servers)
		}
		testsuite.RunParallel(100, init, cleanup, servers, servers)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_CacheExpireTime_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	proxyPool, proxyMgr, certPool := testproxy.PoolAndManager(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	t.Run("part", func(t *testing.T) {
		client := NewClient(certPool, proxyPool)

		get := func() {
			_ = client.GetCacheExpireTime()
		}
		set := func() {
			const expire = 3 * time.Minute

			err := client.SetCacheExpireTime(expire)
			require.NoError(t, err)

			e := client.GetCacheExpireTime()
			require.Equal(t, expire, e)
		}
		testsuite.RunParallel(100, nil, nil, get, get, set, set)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("whole", func(t *testing.T) {
		var client *Client

		init := func() {
			client = NewClient(certPool, proxyPool)
		}
		get := func() {
			_ = client.GetCacheExpireTime()
		}
		set := func() {
			const expire = 3 * time.Minute

			err := client.SetCacheExpireTime(expire)
			require.NoError(t, err)

			e := client.GetCacheExpireTime()
			require.Equal(t, expire, e)
		}
		testsuite.RunParallel(100, init, nil, get, get, set, set)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_Cache_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const domain = "test.com"

	ipv4 := []string{"1.1.1.1", "1.0.0.1"}
	ipv6 := []string{"240c::1111", "240c::1001"}

	proxyPool, proxyMgr, certPool := testproxy.PoolAndManager(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	t.Run("part", func(t *testing.T) {
		client := NewClient(certPool, proxyPool)

		query1 := func() {
			client.queryCache(domain, TypeIPv4)
		}
		query2 := func() {
			client.queryCache(domain, TypeIPv6)
		}
		update1 := func() {
			client.updateCache(domain, TypeIPv4, ipv4)
		}
		update2 := func() {
			client.updateCache(domain, TypeIPv6, ipv6)
		}
		enableCache := func() {
			time.Sleep(time.Duration(3+random.Int(5)) * time.Millisecond)
			client.EnableCache()
		}
		disableCache := func() {
			time.Sleep(time.Duration(3+random.Int(5)) * time.Millisecond)
			client.DisableCache()
		}
		flushCache := func() {
			client.FlushCache()
		}
		cleanup := func() {
			client.FlushCache()
		}
		fns := []func(){
			query1, query2, update1, update2,
			enableCache, disableCache, flushCache,
		}
		testsuite.RunParallel(100, nil, cleanup, fns...)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("whole", func(t *testing.T) {
		var client *Client

		init := func() {
			client = NewClient(certPool, proxyPool)
		}
		query1 := func() {
			client.queryCache(domain, TypeIPv4)
		}
		query2 := func() {
			client.queryCache(domain, TypeIPv6)
		}
		update1 := func() {
			client.updateCache(domain, TypeIPv4, ipv4)
		}
		update2 := func() {
			client.updateCache(domain, TypeIPv6, ipv6)
		}
		enableCache := func() {
			time.Sleep(time.Duration(3+random.Int(5)) * time.Millisecond)
			client.EnableCache()
		}
		disableCache := func() {
			time.Sleep(time.Duration(3+random.Int(5)) * time.Millisecond)
			client.DisableCache()
		}
		flushCache := func() {
			client.FlushCache()
		}
		fns := []func(){
			query1, query2, update1, update2,
			enableCache, disableCache, flushCache,
		}
		testsuite.RunParallel(100, init, nil, fns...)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_Resolve_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	proxyPool, proxyMgr, certPool := testproxy.PoolAndManager(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	t.Run("part", func(t *testing.T) {
		const domain = "api.cloudflare.com"

		client := NewClient(certPool, proxyPool)
		testAddAllDNSServers(t, client)
		client.DisableCache()

		resolve := func(mode, method, log string) {
			opts := Options{
				Mode:   mode,
				Method: method,
			}
			result, err := client.Resolve(domain, &opts)
			require.NoError(t, err)
			require.NotEmpty(t, result)
			t.Log(log+":", result)
		}

		udp := func() {
			resolve(ModeCustom, MethodUDP, "udp")
		}
		tcp := func() {
			resolve(ModeCustom, MethodTCP, "tcp")
		}
		dot := func() {
			resolve(ModeCustom, MethodDoT, "dot")
		}
		doh := func() {
			resolve(ModeCustom, MethodDoH, "doh")
		}
		system := func() {
			resolve(ModeSystem, MethodUDP, "system")
		}
		fns := []func(){
			udp, tcp, dot, doh,
			system,
		}
		testsuite.RunParallel(2, nil, nil, fns...)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("whole", func(t *testing.T) {
		const domain = "dns.cloudflare.com"

		var client *Client

		resolve := func(mode, method, log string) {
			opts := Options{
				Mode:   mode,
				Method: method,
			}
			result, err := client.Resolve(domain, &opts)
			require.NoError(t, err)
			require.NotEmpty(t, result)
			t.Log(log+":", result)
		}

		init := func() {
			client = NewClient(certPool, proxyPool)
			testAddAllDNSServers(t, client)
			client.DisableCache()
		}
		udp := func() {
			resolve(ModeCustom, MethodUDP, "udp")
		}
		tcp := func() {
			resolve(ModeCustom, MethodTCP, "tcp")
		}
		dot := func() {
			resolve(ModeCustom, MethodDoT, "dot")
		}
		doh := func() {
			resolve(ModeCustom, MethodDoH, "doh")
		}
		system := func() {
			resolve(ModeSystem, MethodUDP, "system")
		}
		fns := []func(){
			udp, tcp, dot, doh,
			system,
		}
		testsuite.RunParallel(2, init, nil, fns...)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	proxyPool, proxyMgr, certPool := testproxy.PoolAndManager(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	const (
		tag1 = "test-01"
		tag2 = "test-02"
		tag3 = "test-03"
		tag4 = "test-04"
	)

	server1 := &Server{
		Method:  MethodUDP,
		Address: "127.0.0.1:53",
	}
	server2 := &Server{
		Method:  MethodDoT,
		Address: "127.0.0.1:853",
	}
	server3 := &Server{
		Method:  MethodUDP,
		Address: "127.0.0.2:53",
	}
	server4 := &Server{
		Method:  MethodDoT,
		Address: "127.0.0.2:853",
	}

	t.Run("part", func(t *testing.T) {
		const domain = "api.cloudflare.com"

		client := NewClient(certPool, proxyPool)
		testAddAllDNSServers(t, client)

		resolve := func(mode, method, log string) {
			opts := Options{
				Mode:   mode,
				Method: method,
			}
			result, err := client.Resolve(domain, &opts)
			require.NoError(t, err)
			require.NotEmpty(t, result)
			t.Log(log+":", result)
		}

		init := func() {
			err := client.Add(tag1, server1)
			require.NoError(t, err)
			err = client.Add(tag2, server2)
			require.NoError(t, err)
		}
		add1 := func() {
			err := client.Add(tag3, server3)
			require.NoError(t, err)
		}
		add2 := func() {
			err := client.Add(tag4, server4)
			require.NoError(t, err)
		}
		delete1 := func() {
			err := client.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := client.Delete(tag2)
			require.NoError(t, err)
		}
		servers := func() {
			servers := client.Servers()
			require.NotEmpty(t, servers)
		}
		getCacheExpireTime := func() {
			client.GetCacheExpireTime()
		}
		setCacheExpireTime := func() {
			const expire = 2 * time.Minute

			err := client.SetCacheExpireTime(expire)
			require.NoError(t, err)
			e := client.GetCacheExpireTime()
			require.Equal(t, expire, e)
		}
		enableCache := func() {
			time.Sleep(time.Duration(3+random.Int(5)) * time.Millisecond)
			client.EnableCache()
		}
		disableCache := func() {
			time.Sleep(time.Duration(3+random.Int(5)) * time.Millisecond)
			client.DisableCache()
		}
		flushCache := func() {
			client.FlushCache()
		}
		udp := func() {
			resolve(ModeCustom, MethodUDP, "udp")
		}
		tcp := func() {
			resolve(ModeCustom, MethodTCP, "tcp")
		}
		system := func() {
			resolve(ModeSystem, MethodUDP, "system")
		}
		cleanup := func() {
			err := client.Delete(tag3)
			require.NoError(t, err)
			err = client.Delete(tag4)
			require.NoError(t, err)
			client.FlushCache()
		}
		fns := []func(){
			add1, add2, delete1, delete2, servers,
			udp, tcp, system,
			enableCache, disableCache, flushCache,
			getCacheExpireTime, setCacheExpireTime,
		}
		testsuite.RunParallel(2, init, cleanup, fns...)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("whole", func(t *testing.T) {
		const domain = "dns.cloudflare.com"

		var client *Client

		resolve := func(mode, method, log string) {
			opts := Options{
				Mode:   mode,
				Method: method,
			}
			result, err := client.Resolve(domain, &opts)
			require.NoError(t, err)
			require.NotEmpty(t, result)
			t.Log(log+":", result)
		}

		init := func() {
			client = NewClient(certPool, proxyPool)
			testAddAllDNSServers(t, client)

			err := client.Add(tag1, server1)
			require.NoError(t, err)
			err = client.Add(tag2, server2)
			require.NoError(t, err)
		}
		add1 := func() {
			err := client.Add(tag3, server3)
			require.NoError(t, err)
		}
		add2 := func() {
			err := client.Add(tag4, server4)
			require.NoError(t, err)
		}
		delete1 := func() {
			err := client.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := client.Delete(tag2)
			require.NoError(t, err)
		}
		servers := func() {
			servers := client.Servers()
			require.NotEmpty(t, servers)
		}
		getCacheExpireTime := func() {
			client.GetCacheExpireTime()
		}
		setCacheExpireTime := func() {
			const expire = 2 * time.Minute

			err := client.SetCacheExpireTime(expire)
			require.NoError(t, err)
			e := client.GetCacheExpireTime()
			require.Equal(t, expire, e)
		}
		enableCache := func() {
			time.Sleep(time.Duration(3+random.Int(5)) * time.Millisecond)
			client.EnableCache()
		}
		disableCache := func() {
			time.Sleep(time.Duration(3+random.Int(5)) * time.Millisecond)
			client.DisableCache()
		}
		flushCache := func() {
			client.FlushCache()
		}
		udp := func() {
			resolve(ModeCustom, MethodUDP, "udp")
		}
		tcp := func() {
			resolve(ModeCustom, MethodTCP, "tcp")
		}
		system := func() {
			resolve(ModeSystem, MethodUDP, "system")
		}
		fns := []func(){
			add1, add2, delete1, delete2, servers,
			udp, tcp, system,
			enableCache, disableCache, flushCache,
			getCacheExpireTime, setCacheExpireTime,
		}
		testsuite.RunParallel(2, init, nil, fns...)

		testsuite.IsDestroyed(t, client)
	})
}

func TestServerOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/server.toml")
	require.NoError(t, err)

	// check unnecessary field
	server := Server{}
	err = toml.Unmarshal(data, &server)
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, server)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "udp", actual: server.Method},
		{expected: "1.1.1.1:53", actual: server.Address},
		{expected: true, actual: server.SkipTest},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
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
		{expected: "custom", actual: opts.Mode},
		{expected: "dot", actual: opts.Method},
		{expected: "ipv6", actual: opts.Type},
		{expected: time.Minute, actual: opts.Timeout},
		{expected: "balance", actual: opts.ProxyTag},
		{expected: "cloudflare", actual: opts.ServerTag},
		{expected: "tcp", actual: opts.Network},
		{expected: int64(65536), actual: opts.MaxBodySize},
		{expected: true, actual: opts.SkipProxy},
		{expected: true, actual: opts.SkipTest},
		{expected: "test.com", actual: opts.TLSConfig.ServerName},
		{expected: "keep-alive", actual: opts.Header.Get("Connection")},
		{expected: 2, actual: opts.Transport.MaxIdleConns},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}
