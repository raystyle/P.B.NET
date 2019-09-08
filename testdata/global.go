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
	b, err := ioutil.ReadFile("../config/global/proxyclient.toml")
	require.NoError(t, err)
	err = toml.Unmarshal(b, &clients)
	require.NoError(t, err)
	return clients
}

func DNSServers(t require.TestingT) map[string]*dns.Server {
	servers := make(map[string]*dns.Server)
	b, err := ioutil.ReadFile("../config/global/dnsclient.toml")
	require.NoError(t, err)
	err = toml.Unmarshal(b, &servers)
	require.NoError(t, err)
	return servers
}

func TimeSyncerConfigs(t require.TestingT) map[string]*timesync.Config {
	configs := make(map[string]*timesync.Config)
	b, err := ioutil.ReadFile("../config/global/timesyncer.toml")
	require.NoError(t, err)
	err = toml.Unmarshal(b, &configs)
	require.NoError(t, err)
	return configs
}

func TimeSyncerConfigsFull(t require.TestingT) map[string]*timesync.Config {
	c := make(map[string]*timesync.Config)
	b, err := ioutil.ReadFile("../config/global/timesyncer_full.toml")
	require.NoError(t, err)
	err = toml.Unmarshal(b, &c)
	require.NoError(t, err)
	return c
}
