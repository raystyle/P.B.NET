package dns

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
	"project/internal/testsuite/testproxy"
)

const (
	testIPv4DNS = "dns_ipv4.toml"
	testIPv6DNS = "dns_ipv6.toml"
	testDSDNS   = "dns_ds.toml" // DS: double stack
)

func testAddAllDNSServers(t *testing.T, client *Client) {
	if testsuite.EnableIPv4() {
		testAddDNSServers(t, client, testIPv4DNS)
	}
	if testsuite.EnableIPv6() {
		testAddDNSServers(t, client, testIPv6DNS)
	}
	testAddDNSServers(t, client, testDSDNS)
}

func testAddDNSServers(t *testing.T, client *Client, filename string) {
	servers := make(map[string]*Server)
	b, err := ioutil.ReadFile("testdata/" + filename)
	require.NoError(t, err)
	require.NoError(t, toml.Unmarshal(b, &servers))
	for tag, server := range servers {
		require.NoError(t, client.Add(tag, server))
	}
}

func TestClient(t *testing.T) {
	t.Parallel()

	pool, manager := testproxy.PoolAndManager(t)
	defer func() { require.NoError(t, manager.Close()) }()

	newClient := func(t *testing.T) *Client {
		client := NewClient(pool)
		testAddAllDNSServers(t, client)
		return client
	}

	t.Run("print DNS servers", func(t *testing.T) {
		client := newClient(t)

		for tag, server := range client.Servers() {
			t.Log(tag, server.Address)
		}

		testsuite.IsDestroyed(t, client)
	})

	t.Run("use default options", func(t *testing.T) {
		client := newClient(t)

		result, err := client.Resolve(testDomain, nil)
		require.NoError(t, err)
		require.NotEqual(t, 0, len(result))
		t.Log("use default options", result)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("system mode", func(t *testing.T) {
		client := newClient(t)

		opts := &Options{Mode: ModeSystem}

		if testsuite.EnableIPv4() {
			opts.Type = TypeIPv4
			result, err := client.TestOption(context.Background(), testDomain, opts)
			require.NoError(t, err)
			require.NotEqual(t, 0, len(result))
		}

		if testsuite.EnableIPv6() {
			opts.Type = TypeIPv6
			result, err := client.TestOption(context.Background(), testDomain, opts)
			require.NoError(t, err)
			require.NotEqual(t, 0, len(result))
		}

		// IPv4 and IPv6
		if testsuite.EnableIPv4() || testsuite.EnableIPv6() {
			opts.Type = ""
			result, err := client.TestOption(context.Background(), testDomain, opts)
			require.NoError(t, err)
			require.NotEqual(t, 0, len(result))
		}

		// invalid type
		opts.Type = "foo type"
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Equal(t, 0, len(result))

		testsuite.IsDestroyed(t, client)
	})

	t.Run("resolve IPv6", func(t *testing.T) {
		client := newClient(t)

		result, err := client.Resolve(testDomain, &Options{Type: TypeIPv6})
		require.NoError(t, err)
		require.NotEqual(t, 0, len(result))
		t.Log("resolve IPv6:", result)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("use DoH", func(t *testing.T) {
		client := newClient(t)

		opts := &Options{Method: MethodDoH}
		opts.Transport.TLSClientConfig.InsecureLoadFromSystem = true
		result, err := client.Resolve(testDomain, opts)
		require.NoError(t, err)
		require.NotEqual(t, 0, len(result))
		t.Log("use DoH:", result)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_Cache(t *testing.T) {
	pool, manager := testproxy.PoolAndManager(t)
	defer func() { require.NoError(t, manager.Close()) }()

	client := NewClient(pool)
	testAddAllDNSServers(t, client)

	result, err := client.Resolve(testDomain, nil)
	require.NoError(t, err)
	require.NotEqual(t, 0, len(result))
	t.Log("[no cache]:", result)

	result, err = client.Resolve(testDomain, nil)
	require.NoError(t, err)
	require.NotEqual(t, 0, len(result))
	t.Log("[cache]:", result)

	testsuite.IsDestroyed(t, client)
}

func TestClient_Cancel(t *testing.T) {
	pool, manager := testproxy.PoolAndManager(t)
	defer func() { require.NoError(t, manager.Close()) }()

	client := NewClient(pool)
	testAddAllDNSServers(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	opts := &Options{Method: MethodTCP}
	result, err := client.ResolveWithContext(ctx, testDomain, opts)
	require.Error(t, err)
	require.Equal(t, 0, len(result))

	testsuite.IsDestroyed(t, client)
}

func TestClient_NoResult(t *testing.T) {
	pool, manager := testproxy.PoolAndManager(t)
	defer func() { require.NoError(t, manager.Close()) }()

	client := NewClient(pool)

	if testsuite.EnableIPv4() {
		err := client.Add("reachable-ipv4", &Server{
			Method:  MethodUDP,
			Address: "1.1.1.1:53",
		})
		require.NoError(t, err)
	}

	if testsuite.EnableIPv6() {
		err := client.Add("reachable-ipv6", &Server{
			Method:  MethodUDP,
			Address: "[2606:4700:4700::1111]:53",
		})
		require.NoError(t, err)
	}

	opts := &Options{Method: MethodUDP}
	result, err := client.Resolve("test", opts)
	require.Error(t, err)
	require.Equal(t, 0, len(result))

	testsuite.IsDestroyed(t, client)
}

func TestClient_Add_Delete(t *testing.T) {
	client := NewClient(nil)

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
	t.Parallel()

	pool, manager := testproxy.PoolAndManager(t)
	defer func() { require.NoError(t, manager.Close()) }()

	t.Run("reachable and skip test", func(t *testing.T) {
		client := NewClient(pool)
		opts := new(Options)

		// no DNS server
		result, err := client.TestServers(context.Background(), testDomain, opts)
		require.EqualError(t, err, "no DNS server")
		require.Equal(t, 0, len(result))

		// add reachable and skip test
		if testsuite.EnableIPv4() {
			err := client.Add("reachable-ipv4", &Server{
				Method:  MethodUDP,
				Address: "1.1.1.1:53",
			})
			require.NoError(t, err)
		}
		if testsuite.EnableIPv6() {
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
		require.NotEqual(t, 0, len(result))
		t.Log(result)

		if testsuite.EnableIPv4() {
			require.NoError(t, client.Delete("reachable-ipv4"))
		}
		if testsuite.EnableIPv6() {
			require.NoError(t, client.Delete("reachable-ipv6"))
		}

		testsuite.IsDestroyed(t, client)
	})

	t.Run("unreachable DNS server", func(t *testing.T) {
		client := NewClient(pool)

		err := client.Add("unreachable", &Server{
			Method:  MethodUDP,
			Address: "1.2.3.4",
		})
		require.NoError(t, err)
		result, err := client.TestServers(context.Background(), testDomain, new(Options))
		require.Error(t, err)
		require.Equal(t, 0, len(result))

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_TestOptions(t *testing.T) {
	t.Parallel()

	pool, manager := testproxy.PoolAndManager(t)
	defer func() { require.NoError(t, manager.Close()) }()

	newClient := func(t *testing.T) *Client {
		client := NewClient(pool)
		testAddAllDNSServers(t, client)
		return client
	}

	t.Run("skip test", func(t *testing.T) {
		client := newClient(t)

		opts := &Options{SkipTest: true}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.NoError(t, err)
		require.Equal(t, 0, len(result))

		testsuite.IsDestroyed(t, client)
	})

	t.Run("skip proxy", func(t *testing.T) {
		client := newClient(t)

		opts := &Options{
			ProxyTag:  "tag",
			SkipProxy: true,
		}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.NoError(t, err)
		require.NotEqual(t, 0, len(result))

		testsuite.IsDestroyed(t, client)
	})

	t.Run("invalid domain name", func(t *testing.T) {
		client := newClient(t)

		opts := &Options{Mode: ModeSystem}
		result, err := client.TestOption(context.Background(), "test", opts)
		require.Error(t, err)
		require.Equal(t, 0, len(result))

		testsuite.IsDestroyed(t, client)
	})

	t.Run("with proxy tag", func(t *testing.T) {
		client := newClient(t)

		opts := &Options{
			Method:   MethodTCP, // must don't use udp
			ProxyTag: testproxy.TagBalance,
		}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.NoError(t, err)
		require.NotEqual(t, 0, len(result))

		testsuite.IsDestroyed(t, client)
	})

	t.Run("unknown type", func(t *testing.T) {
		client := newClient(t)

		opts := &Options{Type: "foo type"}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Equal(t, 0, len(result))
		t.Log(err)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("unknown mode", func(t *testing.T) {
		client := newClient(t)

		opts := &Options{Mode: "foo mode"}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Equal(t, 0, len(result))

		testsuite.IsDestroyed(t, client)
	})

	t.Run("unknown method", func(t *testing.T) {
		client := newClient(t)

		opts := &Options{Method: "foo method"}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Equal(t, 0, len(result))

		testsuite.IsDestroyed(t, client)
	})

	t.Run("invalid http transport options", func(t *testing.T) {
		client := newClient(t)

		opts := &Options{Method: MethodDoH}
		opts.Transport.TLSClientConfig.RootCAs = []string{"foo ca"}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Equal(t, 0, len(result))

		// with server tag
		opts.ServerTag = "doh_ipv4_cloudflare"
		result, err = client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Equal(t, 0, len(result))

		testsuite.IsDestroyed(t, client)
	})

	t.Run("doesn't exist proxy", func(t *testing.T) {
		client := newClient(t)

		opts := &Options{ProxyTag: "foo proxy"}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Equal(t, 0, len(result))

		testsuite.IsDestroyed(t, client)
	})

	t.Run("doesn't exist server tag", func(t *testing.T) {
		client := newClient(t)

		opts := &Options{ServerTag: "foo server"}
		result, err := client.TestOption(context.Background(), testDomain, opts)
		require.Error(t, err)
		require.Equal(t, 0, len(result))

		testsuite.IsDestroyed(t, client)
	})
}

func TestOptions(t *testing.T) {
	// DNS Server
	b, err := ioutil.ReadFile("testdata/server.toml")
	require.NoError(t, err)
	server := Server{}
	require.NoError(t, toml.Unmarshal(b, &server))

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

	// Resolve Options
	b, err = ioutil.ReadFile("testdata/options.toml")
	require.NoError(t, err)
	opts := Options{}
	require.NoError(t, toml.Unmarshal(b, &opts))

	testdata = []*struct {
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
