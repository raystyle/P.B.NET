package dns

import (
	"io/ioutil"
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
	"project/internal/testsuite/testproxy"
)

func TestDNSClient(t *testing.T) {
	// make proxy pool
	manager, pool := testproxy.ProxyPoolAndManager(t)
	defer func() { _ = manager.Close() }()
	// create dns servers
	servers := make(map[string]*Server)
	b, err := ioutil.ReadFile("testdata/dnsserver.toml")
	require.NoError(t, err)
	err = toml.Unmarshal(b, &servers)
	require.NoError(t, err)
	// make dns client
	client := NewClient(pool)
	// add
	for tag, server := range servers {
		err = client.Add(tag, server)
		require.NoError(t, err)
	}
	// delete dns server
	err = client.Delete("udp_google")
	require.NoError(t, err)
	// delete doesn't exist
	err = client.Delete("udp_google")
	require.Error(t, err)
	// print servers
	for tag, server := range client.Servers() {
		t.Log(tag, server.Method, server.Address)
	}
	// resolve with default options
	ipList, err := client.Resolve(testDomain, nil)
	require.NoError(t, err)
	t.Log("use default options", ipList)
	// resolve with tag
	opts := Options{ServerTag: "tcp_google"}
	ipList, err = client.Resolve(testDomain, &opts)
	require.NoError(t, err)
	t.Log("with tag", ipList)
	// client.FlushCache()
	client.FlushCache()
	testsuite.IsDestroyed(t, client)
}
