package node

import (
	"bytes"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/testdata"
)

func testGenerateConfig(t *testing.T, isGenesis bool) *Config {
	regAESKey := bytes.Repeat([]byte{0}, aes.Bit256+aes.IVSize)
	cfg := &Config{
		// logger
		LogLevel: "debug",

		// global
		ProxyClients:       testdata.ProxyClients(t),
		DNSServers:         testdata.DNSServers(t),
		DnsCacheDeadline:   3 * time.Minute,
		TimeSyncerConfigs:  testdata.TimeSyncerConfigs(t),
		TimeSyncerInterval: 15 * time.Minute,

		// sender
		MaxBufferSize:   4096,
		SenderWorker:    runtime.NumCPU(),
		SenderQueueSize: 512,

		// syncer
		MaxSyncerClient:  2,
		SyncerWorker:     64,
		SyncerQueueSize:  512,
		ReserveWorker:    16,
		RetryTimes:       3,
		RetryInterval:    5 * time.Second,
		BroadcastTimeout: 30 * time.Second,

		// controller configs
		CtrlPublicKey:   testdata.CtrlED25519.PublicKey(),
		CtrlExPublicKey: testdata.CtrlCurve25519,
		CtrlAESCrypto:   testdata.CtrlAESKey,

		// register
		IsGenesis:      isGenesis,
		RegisterAESKey: regAESKey,

		// server
		ConnLimit: 10,
		Listeners: testdata.Listeners(t),
	}
	// encrypt register info
	register := testdata.Register(t)
	for i := 0; i < len(register); i++ {
		configEnc, err := aes.CBCEncrypt(register[i].Config,
			regAESKey[:aes.Bit256], regAESKey[aes.Bit256:])
		require.NoError(t, err)
		register[i].Config = configEnc
	}
	cfg.RegisterBootstraps = register
	return cfg
}

func TestCheckConfig(t *testing.T) {
	config := testGenerateConfig(t, false)
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
	for k := range node.global.timeSyncer.Configs() {
		t.Log("time syncer config:", k)
	}
}
