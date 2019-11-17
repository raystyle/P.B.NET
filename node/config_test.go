package node

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testGenerateConfig() *Config {
	c := Config{}

	c.Debug.SkipTimeSyncer = true

	c.Logger.Level = "debug"
	c.Logger.Writer = os.Stdout

	c.Global.DNSCacheExpire = 3 * time.Minute
	c.Global.TimeSyncInterval = 1 * time.Minute

	c.Sender.MaxBufferSize = 16384
	c.Sender.Worker = 64
	c.Sender.QueueSize = 512

	c.Syncer.MaxBufferSize = 16384
	c.Syncer.Worker = 64
	c.Syncer.QueueSize = 512
	c.Syncer.ExpireTime = 3 * time.Minute
	return &c
}

func TestConfig_Check(t *testing.T) {
	config := testGenerateConfig()
	output, err := config.Check(context.Background(), nil)
	require.NoError(t, err)
	t.Log(output)
	_, err = New(config)
	require.NoError(t, err)
}
