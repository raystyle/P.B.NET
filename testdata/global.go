package testdata

import (
	"io/ioutil"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/proxy/socks"
	"project/internal/timesync"
)

// ProxyClientTag is the tag of the proxy client in ProxyClients()
const ProxyClientTag = "test_socks5"

// ProxyClients is used to deploy a test proxy server
// and return corresponding proxy client
func ProxyClients(t require.TestingT) []*proxy.Client {
	// deploy test proxy server
	server, err := socks.NewServer("test", logger.Test, nil)
	require.NoError(t, err)
	err = server.ListenAndServe("tcp", "localhost:0")
	require.NoError(t, err)
	return []*proxy.Client{
		{
			Tag:     ProxyClientTag,
			Mode:    proxy.ModeSocks,
			Network: "tcp",
			Address: server.Address(),
		},
	}
}

// DNSServers is used to provide test DNS servers
func DNSServers(t require.TestingT) map[string]*dns.Server {
	servers := make(map[string]*dns.Server)
	files := [...]string{
		"../internal/dns/testdata/dns_ds.toml",
	}
	for i := 0; i < 3; i++ {
		b, err := ioutil.ReadFile(files[i])
		require.NoError(t, err)
		s := make(map[string]*dns.Server)
		require.NoError(t, toml.Unmarshal(b, &s))
		for tag, server := range s {
			servers[tag] = server
		}
	}
	return servers
}

// TimeSyncerClients is used to provide test time syncer clients
func TimeSyncerClients(t require.TestingT) map[string]*timesync.Client {
	clients := make(map[string]*timesync.Client)
	config, err := ioutil.ReadFile("../internal/timesync/testdata/http_opts.toml")
	require.NoError(t, err)
	clients["test_http"] = &timesync.Client{
		Mode:   timesync.ModeHTTP,
		Config: config,
	}
	config, err = ioutil.ReadFile("../internal/timesync/testdata/ntp_opts.toml")
	require.NoError(t, err)
	clients["test_ntp"] = &timesync.Client{
		Mode:   timesync.ModeNTP,
		Config: config,
	}
	return clients
}
