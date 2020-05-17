package node

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/logger"
	"project/internal/patch/toml"
	"project/internal/testsuite"

	"project/testdata"
)

func testGenerateConfig(t testing.TB) *Config {
	cfg := Config{}

	cfg.Test.SkipSynchronizeTime = true

	cfg.Logger.Level = "debug"
	cfg.Logger.QueueSize = 512
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, "Node")

	cfg.Global.DNSCacheExpire = 3 * time.Minute
	cfg.Global.TimeSyncSleepFixed = 15
	cfg.Global.TimeSyncSleepRandom = 10
	cfg.Global.TimeSyncInterval = 1 * time.Minute
	cfg.Global.CertPool = testdata.CertPool(t)
	cfg.Global.ProxyClients = testdata.ProxyClients(t)
	cfg.Global.DNSServers = testdata.DNSServers()
	cfg.Global.TimeSyncerClients = testdata.TimeSyncerClients()

	cfg.Client.Timeout = 15 * time.Second
	cfg.Client.TLSConfig.LoadFromCertPool.LoadPrivateRootCA = true
	cfg.Client.TLSConfig.LoadFromCertPool.LoadPrivateClient = true

	cfg.Register.SleepFixed = 10
	cfg.Register.SleepRandom = 20
	cfg.Register.Skip = true

	cfg.Forwarder.MaxClientConns = 7
	cfg.Forwarder.MaxCtrlConns = 10
	cfg.Forwarder.MaxNodeConns = 8
	cfg.Forwarder.MaxBeaconConns = 128

	cfg.Sender.Worker = 64
	cfg.Sender.QueueSize = 512
	cfg.Sender.MaxBufferSize = 512 << 10
	cfg.Sender.Timeout = 15 * time.Second

	cfg.Syncer.ExpireTime = 30 * time.Second

	cfg.Worker.Number = 16
	cfg.Worker.QueueSize = 1024
	cfg.Worker.MaxBufferSize = 16384

	cfg.Server.MaxConns = 10
	cfg.Server.Timeout = 15 * time.Second

	cfg.Ctrl.KexPublicKey = bytes.Repeat([]byte{255}, curve25519.ScalarSize)
	cfg.Ctrl.PublicKey = bytes.Repeat([]byte{255}, ed25519.PublicKeySize)
	cfg.Ctrl.BroadcastKey = bytes.Repeat([]byte{255}, aes.Key256Bit+aes.IVSize)
	return &cfg
}

func TestConfig(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/config.toml")
	require.NoError(t, err)

	// check unnecessary field
	cfg := Config{}
	err = toml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, cfg)

	tds := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "debug", actual: cfg.Logger.Level},
		{expected: 512, actual: cfg.Logger.QueueSize},
		{expected: true, actual: cfg.Logger.Stdout},

		{expected: 2 * time.Minute, actual: cfg.Global.DNSCacheExpire},
		{expected: uint(15), actual: cfg.Global.TimeSyncSleepFixed},
		{expected: uint(10), actual: cfg.Global.TimeSyncSleepRandom},
		{expected: time.Minute, actual: cfg.Global.TimeSyncInterval},

		{expected: 15 * time.Second, actual: cfg.Client.Timeout},
		{expected: "test", actual: cfg.Client.ProxyTag},
		{expected: "custom", actual: cfg.Client.DNSOpts.Mode},
		{expected: "test.com", actual: cfg.Client.TLSConfig.ServerName},

		{expected: uint(15), actual: cfg.Register.SleepFixed},
		{expected: uint(30), actual: cfg.Register.SleepRandom},
		{expected: true, actual: cfg.Register.Skip},

		{expected: 7, actual: cfg.Forwarder.MaxClientConns},
		{expected: 10, actual: cfg.Forwarder.MaxCtrlConns},
		{expected: 8, actual: cfg.Forwarder.MaxNodeConns},
		{expected: 128, actual: cfg.Forwarder.MaxBeaconConns},

		{expected: 16, actual: cfg.Sender.Worker},
		{expected: 512, actual: cfg.Sender.QueueSize},
		{expected: 16384, actual: cfg.Sender.MaxBufferSize},
		{expected: 15 * time.Second, actual: cfg.Sender.Timeout},

		{expected: 30 * time.Second, actual: cfg.Syncer.ExpireTime},

		{expected: 16, actual: cfg.Worker.Number},
		{expected: 32, actual: cfg.Worker.QueueSize},
		{expected: 16384, actual: cfg.Worker.MaxBufferSize},

		{expected: 100, actual: cfg.Server.MaxConns},
		{expected: 15 * time.Second, actual: cfg.Server.Timeout},

		{expected: "name", actual: cfg.Service.Name},
		{expected: "display name", actual: cfg.Service.DisplayName},
		{expected: "description", actual: cfg.Service.Description},
	}
	for _, td := range tds {
		require.Equal(t, td.expected, td.actual)
	}
}

func TestConfig_Run(t *testing.T) {
	config := testGenerateConfig(t)
	err := config.Run(
		context.Background(),
		os.Stdout,
		&TestOptions{
			Domain: "cloudflare.com",
		})
	require.NoError(t, err)
}

func TestConfig_BuildAndLoad(t *testing.T) {
	// compare configuration
	config := testGenerateConfig(t)
	config.Test.SkipSynchronizeTime = false
	config.Logger.Writer = nil

	cfg := testGenerateConfig(t)
	cfg.Test.SkipSynchronizeTime = false
	cfg.Logger.Writer = nil

	require.Equal(t, config, cfg)

	cfg.Logger.Level = "info"
	require.NotEqual(t, config, cfg)

	// build and load configuration
	data, key, err := config.Build()
	require.NoError(t, err)
	newConfig := new(Config)
	err = newConfig.Load(data, key)
	require.NoError(t, err)
	require.Equal(t, config, newConfig)
}
