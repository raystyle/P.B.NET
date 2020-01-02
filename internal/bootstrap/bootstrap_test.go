package bootstrap

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite/testdns"
	"project/internal/xnet"
)

func testGenerateNodes() []*Node {
	nodes := make([]*Node, 2)
	nodes[0] = &Node{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "127.0.0.1:53123",
	}
	nodes[1] = &Node{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "[::1]:53123",
	}
	return nodes
}

func TestLoad(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()

	ctx := context.Background()

	testdata := [...]*struct {
		mode   string
		config string
	}{
		{mode: ModeHTTP, config: "testdata/http.toml"},
		{mode: ModeDNS, config: "testdata/dns.toml"},
		{mode: ModeDirect, config: "testdata/direct.toml"},
	}
	for _, td := range testdata {
		config, err := ioutil.ReadFile(td.config)
		require.NoError(t, err)
		_, err = Load(ctx, td.mode, config, pool, dnsClient)
		require.NoError(t, err)
	}

	// unknown mode
	_, err := Load(ctx, "foo mode", nil, pool, dnsClient)
	require.EqualError(t, err, "unknown mode: foo mode")

	// invalid config
	_, err = Load(ctx, ModeHTTP, nil, pool, dnsClient)
	require.Error(t, err)
}
