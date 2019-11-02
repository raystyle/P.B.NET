package dns

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/options"
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
	// make DNS client
	manager, pool := testproxy.ProxyPoolAndManager(t)
	defer func() { _ = manager.Close() }()
	client := NewClient(pool)
	testAddAllDNSServers(t, client)

	// print DNS servers
	for tag, server := range client.Servers() {
		t.Log(tag, server.Address)
	}

	// resolve with default options
	ipList, err := client.Resolve(testDomain, nil)
	require.NoError(t, err)
	t.Log("use default options", ipList)

	testsuite.IsDestroyed(t, client)
}

func TestClient_Add_Delete(t *testing.T) {
	// make DNS client
	client := NewClient(nil)

	// add DNS server with unknown method
	err := client.Add("foo tag", &Server{Method: "foo method"})
	require.Error(t, err)
	t.Log("add dns server with unknown method: ", err)

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
}

func TestClient_TestDNSServers(t *testing.T) {
	// make DNS client
	manager, pool := testproxy.ProxyPoolAndManager(t)
	defer func() { _ = manager.Close() }()
	client := NewClient(pool)
	testAddAllDNSServers(t, client)

	// set options
	opts := &Options{
		Type:    TypeIPv4,
		Timeout: 10 * time.Second,
	}
	opts.Transport.TLSClientConfig.InsecureLoadFromSystem = true
	require.NoError(t, client.TestDNSServers(testDomain, opts))

	// test unreachable DNS server
	// delete all DNS servers
	client.servers = make(map[string]*Server)
	err := client.Add("unreachable", &Server{
		Method:  MethodUDP,
		Address: "1.2.3.4",
	})
	require.NoError(t, err)
	require.Error(t, client.TestDNSServers(testDomain, opts))
}

func TestClient_TestOptions(t *testing.T) {
	// make DNS client
	manager, pool := testproxy.ProxyPoolAndManager(t)
	defer func() { _ = manager.Close() }()
	client := NewClient(pool)
	testAddAllDNSServers(t, client)

	// skip test
	opts := &Options{SkipTest: true}
	require.NoError(t, client.TestOptions(testDomain, opts))

	opts.SkipTest = false

	// skip proxy
	opts.ProxyTag = "foo proxy tag"
	opts.SkipProxy = true
	require.NoError(t, client.TestOptions(testDomain, opts))

	opts.SkipProxy = false
	client.FlushCache()

	// test system mode
	opts.Mode = ModeSystem
	require.NoError(t, client.TestOptions(testDomain, opts))

	client.FlushCache()

	// invalid domain name
	opts.Mode = ModeSystem
	require.Error(t, client.TestOptions("asd", opts))

	opts.Mode = ModeCustom

	// with proxy
	opts.Method = MethodTCP // must not use udp
	opts.ProxyTag = testproxy.TagBalance
	require.NoError(t, client.TestOptions(testDomain, opts))

	opts.ProxyTag = ""

	// with cache
	require.NoError(t, client.TestOptions(testDomain, opts))
	client.FlushCache()

	// unknown type
	opts.Type = "foo type"
	err := client.TestOptions(testDomain, opts)
	require.Error(t, err, "unknown type: foo type")
	t.Log(err)

	opts.Type = TypeIPv4

	// unknown mode
	opts.Mode = "foo mode"
	err = client.TestOptions(testDomain, opts)
	require.Error(t, err, "unknown mode: foo mode")
	t.Log(err)

	opts.Mode = ModeCustom

	// unknown method
	opts.Method = "foo method"
	err = client.TestOptions(testDomain, opts)
	require.Error(t, err, "unknown method: foo method")

	// invalid http transport options
	opts.Method = MethodDoH
	opts.Transport.TLSClientConfig.RootCAs = []string{"foo CA"}
	err = client.TestOptions(testDomain, opts)
	require.Error(t, err, "invalid http transport options")

	opts.ServerTag = "doh_cloudflare"
	opts.Method = MethodDoH
	opts.Transport.TLSClientConfig.RootCAs = []string{"foo CA"}
	err = client.TestOptions(testDomain, opts)
	require.Error(t, err, "invalid http transport options")

	opts.ServerTag = ""
	opts.Transport = options.HTTPTransport{}

	// doesn't exist proxy
	opts.ProxyTag = "foo proxy"
	err = client.TestOptions(testDomain, opts)
	require.Error(t, err, "doesn't exist proxy")

	opts.ProxyTag = ""

	// doesn't exist server tag
	opts.ServerTag = "foo server"
	err = client.TestOptions(testDomain, opts)
	require.Error(t, err, "doesn't exist server tag")

	opts.ServerTag = ""

	// no result
	err = client.TestOptions("asd.ads.qwq.aa", opts)
	require.Error(t, err)
}
