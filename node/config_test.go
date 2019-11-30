package node

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
)

func testGenerateConfig() *Config {
	cfg := Config{}

	cfg.Debug.SkipTimeSyncer = true

	cfg.Logger.Level = "debug"
	cfg.Logger.Writer = os.Stdout

	cfg.Global.DNSCacheExpire = 3 * time.Minute
	cfg.Global.TimeSyncInterval = 1 * time.Minute

	cfg.Client.ProxyTag = "balance"
	cfg.Client.Timeout = 15 * time.Second

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

	cfg.CTRL.ExPublicKey = bytes.Repeat([]byte{255}, 32)
	cfg.CTRL.PublicKey = bytes.Repeat([]byte{255}, ed25519.PublicKeySize)
	cfg.CTRL.AESCrypto = bytes.Repeat([]byte{255}, aes.Key256Bit+aes.IVSize)
	return &cfg
}

func TestConfig_Check(t *testing.T) {
	config := testGenerateConfig()
	output, err := config.Check(context.Background(), nil)
	defer fmt.Println(output)
	require.NoError(t, err)
}

func TestConfig(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/config.toml")
	require.NoError(t, err)
	cfg := Config{}
	require.NoError(t, toml.Unmarshal(b, &cfg))

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "debug", actual: cfg.Logger.Level},
		{expected: 512, actual: cfg.Logger.QueueSize},
		{expected: 2 * time.Minute, actual: cfg.Global.DNSCacheExpire},
		{expected: time.Minute, actual: cfg.Global.TimeSyncInterval},
		{expected: "test", actual: cfg.Client.ProxyTag},
		{expected: 15 * time.Second, actual: cfg.Client.Timeout},
		{expected: "custom", actual: cfg.Client.DNSOpts.Mode},
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
		{expected: 10, actual: cfg.Server.MaxConns},
		{expected: 15 * time.Second, actual: cfg.Server.Timeout},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}
