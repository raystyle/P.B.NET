package node

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

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

	cfg.Forwarder.MaxCtrlConns = 3
	cfg.Forwarder.MaxNodeConns = 10
	cfg.Forwarder.MaxBeaconConns = 64

	cfg.Sender.Worker = 64
	cfg.Sender.QueueSize = 512
	cfg.Sender.MaxBufferSize = 512 << 10
	cfg.Sender.Timeout = 15 * time.Second

	cfg.Syncer.ExpireTime = 5 * time.Minute

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
	require.NoError(t, err)
	t.Log(output)
	_, err = New(config)
	require.NoError(t, err)
}
