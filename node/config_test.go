package node

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testGenerateConfig() *Config {
	c := Config{
		Debug: Debug{
			SkipTimeSyncer: true,
		},
		LogLevel: "debug",
	}

	c.Global.DNSCacheExpire = 3 * time.Minute
	c.Global.TimeSyncInterval = 1 * time.Minute

	c.Sender.MaxBufferSize = 16384
	c.Sender.Worker = 64
	c.Sender.QueueSize = 512
	c.Sender.MaxConns = 3

	c.Syncer.MaxBufferSize = 16384
	c.Syncer.Worker = 64
	c.Syncer.QueueSize = 512
	c.Syncer.ExpireTime = 3 * time.Minute
	return &c
}

func TestConfig_Check(t *testing.T) {
	config := testGenerateConfig()
	err := config.Check()
	require.NoError(t, err)
	node, err := New(config)
	require.NoError(t, err)
	for k := range node.global.proxyPool.Clients() {
		t.Log("proxy client:", k)
	}
	for k := range node.global.dnsClient.Servers() {
		t.Log("dns server:", k)
	}
	for k := range node.global.timeSyncer.Clients() {
		t.Log("time syncer config:", k)
	}
}
