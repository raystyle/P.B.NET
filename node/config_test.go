package node

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
	cfg.Forwarder.MaxBeaconConns = 16

	cfg.Sender.Worker = 64
	cfg.Sender.QueueSize = 512
	cfg.Sender.MaxBufferSize = 16384
	cfg.Sender.Timeout = 10 * time.Second

	cfg.Syncer.ExpireTime = 3 * time.Minute

	cfg.Worker.Number = 16
	cfg.Worker.QueueSize = 1024
	cfg.Worker.MaxBufferSize = 16384

	cfg.Server.MaxConns = 10
	cfg.Server.Timeout = 15 * time.Second

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
