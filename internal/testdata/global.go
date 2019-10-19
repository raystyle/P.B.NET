package testdata

import (
	"io/ioutil"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/proxy"
	"project/internal/timesync"
)

func ProxyClients(t require.TestingT) map[string]*proxy.Client {
	clients := make(map[string]*proxy.Client)
	config, err := ioutil.ReadFile("../internal/proxy/testdata/socks5.toml")
	require.NoError(t, err)
	clients["socks5"] = &proxy.Client{
		Mode:   proxy.Socks5,
		Config: config,
	}
	config, err = ioutil.ReadFile("../internal/proxy/testdata/http.txt")
	require.NoError(t, err)
	clients["http"] = &proxy.Client{
		Mode:   proxy.HTTP,
		Config: config,
	}
	return clients
}

func DNSServers(t require.TestingT) map[string]*dns.Server {
	servers := make(map[string]*dns.Server)
	b, err := ioutil.ReadFile("../internal/dns/testdata/dnsclient.toml")
	require.NoError(t, err)
	err = toml.Unmarshal(b, &servers)
	require.NoError(t, err)
	return servers
}

func TimeSyncerClients(t require.TestingT) map[string]*timesync.Client {
	clients := make(map[string]*timesync.Client)
	config, err := ioutil.ReadFile("../internal/timesync/testdata/http.toml")
	require.NoError(t, err)
	clients["http"] = &timesync.Client{
		Mode:   timesync.HTTP,
		Config: config,
	}
	config, err = ioutil.ReadFile("../internal/timesync/testdata/ntp.toml")
	require.NoError(t, err)
	clients["ntp"] = &timesync.Client{
		Mode:   timesync.NTP,
		Config: config,
	}
	return clients
}
