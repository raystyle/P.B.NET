package node

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/logger"
	"project/testdata"
)

func testGenerateConfig(tb testing.TB) *Config {
	cfg := Config{}

	cfg.Test.SkipSynchronizeTime = true

	cfg.Logger.Level = "debug"
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, "Node")

	cfg.Global.DNSCacheExpire = 3 * time.Minute
	cfg.Global.TimeSyncSleepFixed = 15
	cfg.Global.TimeSyncSleepRandom = 10
	cfg.Global.TimeSyncInterval = 1 * time.Minute
	cfg.Global.Certificates = testdata.Certificates(tb)
	cfg.Global.ProxyClients = testdata.ProxyClients(tb)
	cfg.Global.DNSServers = testdata.DNSServers()
	cfg.Global.TimeSyncerClients = testdata.TimeSyncerClients()

	cfg.Client.ProxyTag = "balance"
	cfg.Client.Timeout = 15 * time.Second

	cfg.Register.SleepFixed = 10
	cfg.Register.SleepRandom = 20

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

	cfg.CTRL.KexPublicKey = bytes.Repeat([]byte{255}, 32)
	cfg.CTRL.PublicKey = bytes.Repeat([]byte{255}, ed25519.PublicKeySize)
	cfg.CTRL.BroadcastKey = bytes.Repeat([]byte{255}, aes.Key256Bit+aes.IVSize)
	return &cfg
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

func TestConfig_Build_Load(t *testing.T) {
	config := testGenerateConfig(t)
	config.Test.SkipSynchronizeTime = false
	config.Logger.Writer = nil

	cfg := testGenerateConfig(t)
	cfg.Test.SkipSynchronizeTime = false
	cfg.Logger.Writer = nil

	require.Equal(t, config, cfg)

	cfg.Logger.Level = "info"
	require.NotEqual(t, config, cfg)

	built, err := config.Build()
	require.NoError(t, err)
	newConfig := new(Config)
	err = newConfig.Load(built)
	require.NoError(t, err)
	require.Equal(t, config, newConfig)
}

func TestConfig(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/config.toml")
	require.NoError(t, err)
	cfg := Config{}
	require.NoError(t, toml.Unmarshal(b, &cfg))

	tds := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "debug", actual: cfg.Logger.Level},
		{expected: 512, actual: cfg.Logger.QueueSize},

		{expected: 2 * time.Minute, actual: cfg.Global.DNSCacheExpire},
		{expected: uint(15), actual: cfg.Global.TimeSyncSleepFixed},
		{expected: uint(10), actual: cfg.Global.TimeSyncSleepRandom},
		{expected: time.Minute, actual: cfg.Global.TimeSyncInterval},

		{expected: "test", actual: cfg.Client.ProxyTag},
		{expected: 15 * time.Second, actual: cfg.Client.Timeout},
		{expected: "custom", actual: cfg.Client.DNSOpts.Mode},

		{expected: uint(15), actual: cfg.Register.SleepFixed},
		{expected: uint(30), actual: cfg.Register.SleepRandom},
		{expected: true, actual: cfg.Register.Skip},

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
