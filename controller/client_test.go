package controller

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/crypto/cert"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/option"
	"project/internal/protocol"
	"project/internal/testsuite"
	"project/internal/xnet"
	"project/node"
	"project/testdata"
)

func testGenerateNodeConfig(tb testing.TB) *node.Config {
	cfg := node.Config{}

	cfg.Test.SkipSynchronizeTime = true
	cfg.Test.BroadcastTestMsg = make(chan []byte, 4)
	cfg.Test.SendTestMsg = make(chan []byte, 4)

	cfg.Logger.Level = "debug"
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, "Node")

	cfg.Global.DNSCacheExpire = 3 * time.Minute
	cfg.Global.TimeSyncSleepFixed = 15
	cfg.Global.TimeSyncSleepRandom = 10
	cfg.Global.TimeSyncInterval = 1 * time.Minute

	var certificates [][]byte
	for _, pair := range ctrl.GetSelfCerts() {
		c := make([]byte, len(pair.ASN1Data))
		copy(c, pair.ASN1Data)
		certificates = append(certificates, c)
	}
	for _, pair := range ctrl.GetSystemCerts() {
		c := make([]byte, len(pair.ASN1Data))
		copy(c, pair.ASN1Data)
		certificates = append(certificates, c)
	}
	cfg.Global.Certificates = certificates
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
	cfg.Sender.MaxBufferSize = 16 << 10
	cfg.Sender.Timeout = 15 * time.Second

	cfg.Syncer.ExpireTime = 3 * time.Second

	cfg.Worker.Number = 16
	cfg.Worker.QueueSize = 1024
	cfg.Worker.MaxBufferSize = 16 << 10

	cfg.Server.MaxConns = 16 * runtime.NumCPU()
	cfg.Server.Timeout = 15 * time.Second

	cfg.CTRL.ExPublicKey = ctrl.global.KeyExchangePub()
	cfg.CTRL.PublicKey = ctrl.global.PublicKey()
	cfg.CTRL.BroadcastKey = ctrl.global.BroadcastKey()
	return &cfg
}

const testInitialNodeListenerTag = "test_tcp"

func testGenerateInitialNode(t testing.TB) *node.Node {
	cfg := testGenerateNodeConfig(t)
	cfg.Register.Skip = true

	// generate certificate
	certs := ctrl.global.GetSelfCerts()
	opts := cert.Options{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	caCert := certs[0].Certificate
	caKey := certs[0].PrivateKey
	pair, err := cert.Generate(caCert, caKey, &opts)
	require.NoError(t, err)

	// generate listener config
	listener := messages.Listener{
		Tag:     testInitialNodeListenerTag,
		Mode:    xnet.ModeTCP,
		Network: "tcp",
		Address: "localhost:0",
	}
	c, k := pair.EncodeToPEM()
	listener.TLSConfig.Certificates = []option.X509KeyPair{
		{Cert: string(c), Key: string(k)},
	}

	// set node config
	config, key, err := GenerateNodeConfigAboutListeners(&listener)
	require.NoError(t, err)
	cfg.Server.Listeners = config
	cfg.Server.ListenersKey = key

	NODE, err := node.New(cfg)
	require.NoError(t, err)
	testsuite.IsDestroyed(t, cfg)
	go func() {
		err := NODE.Main()
		require.NoError(t, err)
	}()
	NODE.Wait()
	return NODE
}

func testGenerateClient(tb testing.TB, node *node.Node) *client {
	listener, err := node.GetListener(testInitialNodeListenerTag)
	require.NoError(tb, err)
	n := &bootstrap.Node{
		Mode:    xnet.ModeTCP,
		Network: "tcp",
		Address: listener.Addr().String(),
	}
	client, err := ctrl.newClient(context.Background(), n, nil, nil)
	require.NoError(tb, err)
	return client
}

func TestClient_Send(t *testing.T) {
	testInitializeController(t)
	NODE := testGenerateInitialNode(t)
	defer NODE.Exit(nil)
	client := testGenerateClient(t, NODE)
	data := bytes.Buffer{}
	for i := 0; i < 1024; i++ {
		data.Write(convert.Int32ToBytes(int32(i)))
		reply, err := client.Send(protocol.TestCommand, data.Bytes())
		require.NoError(t, err)
		require.Equal(t, data.Bytes(), reply)
		data.Reset()
	}
	client.Close()
	testsuite.IsDestroyed(t, client)
}

func TestClient_SendParallel(t *testing.T) {
	testInitializeController(t)
	NODE := testGenerateInitialNode(t)
	defer NODE.Exit(nil)
	client := testGenerateClient(t, NODE)
	wg := sync.WaitGroup{}
	send := func() {
		data := bytes.Buffer{}
		for i := 0; i < 1024; i++ {
			data.Write(convert.Int32ToBytes(int32(i)))
			reply, err := client.Send(protocol.TestCommand, data.Bytes())
			require.NoError(t, err)
			require.Equal(t, data.Bytes(), reply)
			data.Reset()
		}
		wg.Done()
	}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go send()
	}
	wg.Wait()
	client.Close()
	testsuite.IsDestroyed(t, client)
}

func BenchmarkClient_Send(b *testing.B) {
	testInitializeController(b)
	NODE := testGenerateInitialNode(b)
	defer NODE.Exit(nil)
	client := testGenerateClient(b, NODE)
	data := bytes.Buffer{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data.Write(convert.Int32ToBytes(int32(i)))
		// _, _ = client.Send(protocol.TestCommand, data.Bytes())
		reply, err := client.Send(protocol.TestCommand, data.Bytes())
		require.NoError(b, err)
		require.Equal(b, data.Bytes(), reply)
		data.Reset()
	}
	b.StopTimer()
	client.Close()
	testsuite.IsDestroyed(b, client)
}

func BenchmarkClient_SendParallel(b *testing.B) {
	testInitializeController(b)
	NODE := testGenerateInitialNode(b)
	defer NODE.Exit(nil)
	client := testGenerateClient(b, NODE)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		data := bytes.Buffer{}
		i := 0
		for pb.Next() {
			data.Write(convert.Int32ToBytes(int32(i)))
			// _, _ = client.Send(protocol.TestCommand, data.Bytes())
			reply, err := client.Send(protocol.TestCommand, data.Bytes())
			require.NoError(b, err)
			require.Equal(b, data.Bytes(), reply)
			data.Reset()
			i++
		}
	})
	b.StopTimer()
	client.Close()
	testsuite.IsDestroyed(b, client)
}
