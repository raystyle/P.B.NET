package bootstrap

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite/testdns"
	"project/internal/xnet"
)

func TestListener_String(t *testing.T) {
	listener := Listener{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "127.0.0.1:443",
	}
	expect := "tls (tcp 127.0.0.1:443)"
	require.Equal(t, expect, listener.String())
}

func testGenerateListeners() []*Listener {
	listeners := make([]*Listener, 2)
	listeners[0] = &Listener{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "127.0.0.1:53123",
	}
	listeners[1] = &Listener{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "[::1]:53123",
	}
	return listeners
}

func TestLoad(t *testing.T) {
	dnsClient, proxyPool, manager := testdns.DNSClient(t)
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
		_, err = Load(ctx, td.mode, config, proxyPool, dnsClient)
		require.NoError(t, err)
	}

	// unknown mode
	_, err := Load(ctx, "foo mode", nil, proxyPool, dnsClient)
	require.EqualError(t, err, "unknown mode: foo mode")

	// invalid config
	_, err = Load(ctx, ModeHTTP, nil, proxyPool, dnsClient)
	require.Error(t, err)
}
