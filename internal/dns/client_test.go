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
	"project/internal/testsuite"
	"project/internal/testsuite/testproxy"
)

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

func testAddAllDNSServers(t *testing.T, client *Client) {
	if testsuite.IPv4Enabled {
		testAddDNSServers(t, client, "dns_ipv4.toml")
	}
	if testsuite.IPv6Enabled {
		testAddDNSServers(t, client, "dns_ipv6.toml")
	}
	// DS: double stack
	testAddDNSServers(t, client, "dns_ds.toml")
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

	t.Run("print DNS servers", func(t *testing.T) {
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
			require.Len(t, result, 0)
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
		require.Len(t, result, 0)
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
	require.Len(t, result, 0)

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
	require.Len(t, result, 0)

	testsuite.IsDestroyed(t, client)
}

func TestClient_Add_Delete(t *testing.T) {
	client := NewClient(nil, nil)

	// add DNS server with unknown method
	err := client.Add("foo tag", &Server{Method: "foo method"})
	require.Error(t, err)
	t.Log(err)

	// add exist DNS server
	const tag = "test"
	err = client.Add(tag, &Server{Method: MethodUDP})
	require.NoError(t, err)
	err = client.Add(tag, &Server{Method: MethodUDP})
	require.Error(t, err)

	// delete DNS server
	err = client.Delete(tag)
	require.NoError(t, err)

	// delete doesn't exist DNS server
	err = client.Delete(tag)
	require.Error(t, err)

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

	t.Run("reachable and skip test", func(t *testing.T) {
		client := NewClient(certPool, proxyPool)

		opts := new(Options)

		// no DNS server
		result, err := client.TestServers(context.Background(), testDomain, opts)
		require.Equal(t, err, ErrNoDNSServers)
		require.Len(t, result, 0)

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

		result, err = client.TestServers(context.Background(), testDomain, opts)
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
		result, err := client.TestServers(context.Background(), testDomain, new(Options))
		require.Error(t, err)
		require.Len(t, result, 0)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("cancel", func(t *testing.T) {
		client := NewClient(certPool, proxyPool)
		testAddAllDNSServers(t, client)

		ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()
		result, err := client.TestServers(ctx, testDomain, new(Options))
		require.Error(t, err)
		require.Len(t, result, 0)

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

		result, err := client.TestServers(context.Background(), testDomain, new(Options))
		require.Error(t, err)
		require.Len(t, result, 0)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_TestOptions(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	proxyPool, proxyMgr, certPool := testproxy.PoolAndManager(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
	client := NewClient(certPool, proxyPool)
	testAddAllDNSServers(t, client)

	t.Run("skip test", func(t *testing.T) {
		opts := &Options{SkipTest: true}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.NoError(t, err)
		require.Len(t, result, 0)
	})

	t.Run("skip proxy", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{
			ProxyTag:  "tag",
			SkipProxy: true,
		}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.NoError(t, err)
		require.NotEmpty(t, result)
	})

	t.Run("invalid domain name", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{Mode: ModeSystem}
		result, err := client.TestOption(context.Background(), "test", opts)
		require.Error(t, err)
		require.Len(t, result, 0)
	})

	t.Run("with proxy tag", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{
			Method:   MethodTCP, // must don't use udp
			ProxyTag: testproxy.TagBalance,
		}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.NoError(t, err)
		require.NotEmpty(t, result)
	})

	t.Run("unknown type", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{Type: "foo type"}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Len(t, result, 0)
		t.Log(err)
	})

	t.Run("unknown mode", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{Mode: "foo mode"}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Len(t, result, 0)
	})

	t.Run("unknown method", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{Method: "foo method"}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Len(t, result, 0)
	})

	t.Run("invalid http transport options", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{Method: MethodDoH}
		opts.Transport.TLSClientConfig.RootCAs = []string{"foo ca"}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Len(t, result, 0)

		// with server tag
		opts.ServerTag = "doh_ipv4_cloudflare"
		result, err = client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Len(t, result, 0)
	})

	t.Run("doesn't exist proxy", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{ProxyTag: "foo proxy"}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Len(t, result, 0)
	})

	t.Run("doesn't exist server tag", func(t *testing.T) {
		client.FlushCache()

		opts := &Options{ServerTag: "foo server"}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Len(t, result, 0)
	})

	testsuite.IsDestroyed(t, client)
}

func TestClient_Parallel(t *testing.T) {
	client := NewClient(nil, nil)
	const (
		tag1 = "test-01"
		tag2 = "test-02"
	)

	t.Run("simple", func(t *testing.T) {
		add1 := func() {
			err := client.Add(tag1, &Server{
				Method:  MethodUDP,
				Address: "127.0.0.1:1080",
			})
			require.NoError(t, err)
		}
		add2 := func() {
			err := client.Add(tag2, &Server{
				Method:  MethodUDP,
				Address: "127.0.0.1:1081",
			})
			require.NoError(t, err)
		}
		testsuite.RunParallel(add1, add2)

		get1 := func() {
			servers := client.Servers()
			require.Len(t, servers, 2)
		}
		get2 := func() {
			servers := client.Servers()
			require.Len(t, servers, 2)
		}
		testsuite.RunParallel(get1, get2)

		delete1 := func() {
			err := client.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := client.Delete(tag2)
			require.NoError(t, err)
		}
		testsuite.RunParallel(delete1, delete2)

		require.Len(t, client.Servers(), 0)
	})

	t.Run("mixed", func(t *testing.T) {
		add := func() {
			err := client.Add(tag1, &Server{
				Method:  MethodUDP,
				Address: "127.0.0.1:1080",
			})
			require.NoError(t, err)
		}
		get := func() {
			_ = client.Servers()
		}
		del := func() {
			_ = client.Delete(tag1)
		}
		testsuite.RunParallel(add, get, del)
	})

	t.Run("cache", func(t *testing.T) {
		f1 := func() {
			client.GetCacheExpireTime()
		}
		f2 := func() {
			err := client.SetCacheExpireTime(time.Minute)
			require.NoError(t, err)
		}
		f3 := func() {
			client.FlushCache()
		}
		testsuite.RunParallel(f1, f2, f3)
	})

	testsuite.IsDestroyed(t, client)
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

	testdata := []*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "udp", actual: server.Method},
		{expected: "1.1.1.1:53", actual: server.Address},
		{expected: true, actual: server.SkipTest},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
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

	testdata := []*struct {
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
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}
