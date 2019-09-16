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
	"project/internal/config"
	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/cert"
	"project/internal/options"
	"project/internal/protocol"
	"project/internal/xnet"
	"project/node"
	"project/testdata"
)

func testGenerateNode(t require.TestingT, genesis bool) *node.NODE {
	regAESKey := bytes.Repeat([]byte{0}, aes.Bit256+aes.IVSize)
	cfg := &node.Config{
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
		IsGenesis:      genesis,
		RegisterAESKey: regAESKey,

		// server
		ConnLimit: 10,
		Listeners: testdata.Listeners(t),
	}
	cfg.Debug.SkipTimeSyncer = true
	// encrypt register info
	register := testdata.Register(t)
	for i := 0; i < len(register); i++ {
		configEnc, err := aes.CBCEncrypt(register[i].Config,
			regAESKey[:aes.Bit256], regAESKey[aes.Bit256:])
		require.NoError(t, err)
		register[i].Config = configEnc
	}
	cfg.RegisterBootstraps = register
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
		Mode: xnet.TLS,
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
	cfg := &clientCfg{
		Node: &bootstrap.Node{
			Mode:    xnet.TLS,
			Network: "tcp",
			Address: "localhost:62300",
		},
	}
	client, err := newClient(ctrl, cfg)
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
