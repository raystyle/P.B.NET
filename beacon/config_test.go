package beacon

import (
	"bytes"
	"crypto/ed25519"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/logger"

	"project/testdata"
)

func testGenerateConfig(tb testing.TB) *Config {
	cfg := Config{}

	cfg.Test.SkipSynchronizeTime = true

	cfg.Logger.Level = "debug"
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, "Beacon")

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

	cfg.Sender.Worker = 64
	cfg.Sender.QueueSize = 512
	cfg.Sender.MaxBufferSize = 512 << 10
	cfg.Sender.Timeout = 15 * time.Second

	cfg.Syncer.ExpireTime = 30 * time.Second

	cfg.Worker.Number = 16
	cfg.Worker.QueueSize = 1024
	cfg.Worker.MaxBufferSize = 16384

	cfg.CTRL.KexPublicKey = bytes.Repeat([]byte{255}, curve25519.ScalarSize)
	cfg.CTRL.PublicKey = bytes.Repeat([]byte{255}, ed25519.PublicKeySize)
	cfg.CTRL.BroadcastKey = bytes.Repeat([]byte{255}, aes.Key256Bit+aes.IVSize)
	return &cfg
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

		{expected: 7, actual: cfg.Sender.MaxConns},
		{expected: 16, actual: cfg.Sender.Worker},
		{expected: 512, actual: cfg.Sender.QueueSize},
		{expected: 16384, actual: cfg.Sender.MaxBufferSize},
		{expected: 15 * time.Second, actual: cfg.Sender.Timeout},

		{expected: 30 * time.Second, actual: cfg.Syncer.ExpireTime},

		{expected: 16, actual: cfg.Worker.Number},
		{expected: 32, actual: cfg.Worker.QueueSize},
		{expected: 16384, actual: cfg.Worker.MaxBufferSize},

		{expected: "name", actual: cfg.Service.Name},
		{expected: "display name", actual: cfg.Service.DisplayName},
		{expected: "description", actual: cfg.Service.Description},
	}
	for _, td := range tds {
		require.Equal(t, td.expected, td.actual)
	}
}
