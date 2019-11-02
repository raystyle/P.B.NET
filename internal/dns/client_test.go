package dns

import (
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
}
