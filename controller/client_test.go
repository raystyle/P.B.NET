package controller

import (
	"bytes"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/crypto/cert"
	"project/internal/options"
	"project/internal/protocol"
	"project/internal/testdata"
	"project/internal/xnet"
	"project/node"
)

func testGenerateNodeConfig(t require.TestingT, genesis bool) *node.Config {
	cfg := node.Config{
		// logger
		LogLevel: "debug",

		// global
		ProxyClients:       testdata.ProxyClients(t),
		DNSServers:         testdata.DNSServers(t),
		DnsCacheDeadline:   3 * time.Minute,
		TimeSyncerConfigs:  testdata.TimeSyncerClients(t),
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
		CtrlAESCrypto:   testdata.CtrlBroadcastKey,

		// register
		IsGenesis:      genesis,
		RegisterAESKey: regAESKey,

		// server
		ConnLimit: 10,
	}
	cfg.Debug.SkipTimeSyncer = true

	return &cfg
}

func testGenerateNode(t require.TestingT, genesis bool) *node.Node {
	cfg := testGenerateNodeConfig(t, genesis)
	NODE, err := node.New(cfg)
	require.NoError(t, err)
	go func() {
		err := NODE.Main()
		require.NoError(t, err)
	}()
	NODE.TestWait()
	// generate listener config
	listenerCfg := config.Listener{
		Tag:  "test_tls_listener",
		Mode: xnet.ModeTLS,
	}
	xnetCfg := xnet.Config{
		Network: "tcp",
		Address: "localhost:62300",
	}
	// generate node certificate
	caCert := ctrl.global.CACertificates()
	caPri := ctrl.global.CAPrivateKeys()
	certCfg := cert.Config{DNSNames: []string{"localhost"}}
	sCert, sPri, err := cert.Generate(caCert[0], caPri[0], &certCfg)
	require.NoError(t, err)
	kp := options.X509KeyPair{Cert: string(sCert), Key: string(sPri)}
	xnetCfg.TLSConfig.Certificates = []options.X509KeyPair{kp}
	// set config
	listenerCfg.Config, err = toml.Marshal(&xnetCfg)
	require.NoError(t, err)
	require.NoError(t, NODE.AddListener(&listenerCfg))
	return NODE
}

func testGenerateClient(t require.TestingT) *client {
	n := &bootstrap.Node{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "localhost:62300",
	}
	client, err := newClient(ctrl, n, nil, nil)
	require.NoError(t, err)
	return client
}

func TestClient_Send(t *testing.T) {
	testInitCtrl(t)
	NODE := testGenerateNode(t, true)
	defer NODE.Exit(nil)
	client := testGenerateClient(t)
	data := bytes.Repeat([]byte{1}, 128)
	reply, err := client.Send(protocol.TestCommand, data)
	require.NoError(t, err)
	require.Equal(t, data, reply)
	client.Close()
}

func TestClient_SendParallel(t *testing.T) {
	testInitCtrl(t)
	NODE := testGenerateNode(t, true)
	defer NODE.Exit(nil)
	client := testGenerateClient(t)
	wg := sync.WaitGroup{}
	send := func() {
		data := bytes.NewBuffer(nil)
		for i := 0; i < 1024; i++ {
			data.Write(convert.Int32ToBytes(int32(i)))
			reply, err := client.Send(protocol.TestCommand, data.Bytes())
			require.NoError(t, err)
			require.Equal(t, data.Bytes(), reply)
			data.Reset()
		}
		wg.Done()
	}
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go send()
	}
	wg.Wait()
	client.Close()
}

func BenchmarkClient_Send(b *testing.B) {
	testInitCtrl(b)
	NODE := testGenerateNode(b, true)
	defer NODE.Exit(nil)
	client := testGenerateClient(b)
	data := bytes.NewBuffer(nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data.Write(convert.Int32ToBytes(int32(b.N)))
		_, _ = client.Send(protocol.TestCommand, data.Bytes())
		// reply, err := client.Send(protocol.TestCommand, data.Bytes())
		// require.NoError(b, err)
		// require.Equal(b, data.Bytes(), reply)
		data.Reset()
	}
	b.StopTimer()
	client.Close()
}

func BenchmarkClient_SendParallel(b *testing.B) {
	testInitCtrl(b)
	NODE := testGenerateNode(b, true)
	defer NODE.Exit(nil)
	client := testGenerateClient(b)
	nOnce := b.N / runtime.NumCPU()
	wg := sync.WaitGroup{}
	send := func() {
		data := bytes.NewBuffer(nil)
		for i := 0; i < nOnce; i++ {
			data.Write(convert.Int32ToBytes(int32(i)))
			_, _ = client.Send(protocol.TestCommand, data.Bytes())
			// reply, err := client.Send(protocol.TestCommand, data.Bytes())
			// require.NoError(b, err)
			// require.Equal(b, data.Bytes(), reply)
			data.Reset()
		}
		wg.Done()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go send()
	}
	wg.Wait()
	b.StopTimer()
	client.Close()
}
