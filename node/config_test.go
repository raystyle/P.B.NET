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

	cfg.Sender.MaxBufferSize = 16384
	cfg.Sender.Worker = 64
	cfg.Sender.QueueSize = 512

	cfg.Syncer.MaxBufferSize = 16384
	cfg.Syncer.Worker = 64
	cfg.Syncer.QueueSize = 512
	cfg.Syncer.ExpireTime = 3 * time.Minute
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
