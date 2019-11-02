package dns

import (
	"io/ioutil"
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
	"project/internal/testsuite/testproxy"
)

func TestClient(t *testing.T) {
	// make proxy pool
	manager, pool := testproxy.ProxyPoolAndManager(t)
	defer func() { _ = manager.Close() }()

	// make dns client
	client := NewClient(pool)

	// add dns servers
	servers := make(map[string]*Server)
	b, err := ioutil.ReadFile("testdata/dnsserver.toml")
	require.NoError(t, err)
	err = toml.Unmarshal(b, &servers)
	require.NoError(t, err)
	for tag, server := range servers {
		err = client.Add(tag, server)
		require.NoError(t, err)
	}

	// print dns servers
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
	// make dns client
	client := NewClient(nil)

	// add dns server with unknown method
	err := client.Add("foo tag", &Server{Method: "foo method"})
	require.Error(t, err)
	t.Log("add dns server with unknown method: ", err)

	// add exist
	const tag = "test"
	err = client.Add(tag, &Server{Method: MethodUDP})
	require.NoError(t, err)
	err = client.Add(tag, &Server{Method: MethodUDP})
	require.Error(t, err)

	// delete dns server
	err = client.Delete(tag)
	require.NoError(t, err)

	// delete doesn't exist
	err = client.Delete(tag)
	require.Error(t, err)
}
