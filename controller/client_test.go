package controller

import (
	"bytes"
	"context"
	"crypto/tls"
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

func testGenerateNodeConfig(t testing.TB) *node.Config {
	cfg := node.Config{}

	cfg.Test.SkipSynchronizeTime = true

	cfg.Logger.Level = "debug"
	cfg.Logger.QueueSize = 512
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, "Node")

	cfg.Global.DNSCacheExpire = 3 * time.Minute
	cfg.Global.TimeSyncSleepFixed = 15
	cfg.Global.TimeSyncSleepRandom = 10
	cfg.Global.TimeSyncInterval = 1 * time.Minute

	ctrl.global.CertPool.AddToRawCertPool(&cfg.Global.RawCertPool)
	cfg.Global.ProxyClients = testdata.ProxyClients(t)
	cfg.Global.DNSServers = testdata.DNSServers()
	cfg.Global.TimeSyncerClients = testdata.TimeSyncerClients()

	cfg.Client.Timeout = 15 * time.Second

	cfg.Register.SleepFixed = 10
	cfg.Register.SleepRandom = 20

	cfg.Forwarder.MaxClientConns = 7
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

	cfg.Ctrl.KexPublicKey = ctrl.global.KeyExchangePublicKey()
	cfg.Ctrl.PublicKey = ctrl.global.PublicKey()
	cfg.Ctrl.BroadcastKey = ctrl.global.BroadcastKey()
	return &cfg
}

const testInitialNodeListenerTag = "initial_tls"

func testGenerateInitialNode(t testing.TB) *node.Node {
	testInitializeController(t)

	cfg := testGenerateNodeConfig(t)
	cfg.Register.Skip = true

	// generate certificate
	certs := ctrl.global.CertPool.GetPrivateRootCAPairs()
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
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "localhost:0",
	}
	certPEM, keyPEM := pair.EncodeToPEM()
	listener.TLSConfig.Certificates = []option.X509KeyPair{
		{Cert: string(certPEM), Key: string(keyPEM)},
	}
	listener.TLSConfig.LoadFromCertPool.LoadPrivateClientCACerts = true
	listener.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert

	// set node config
	data, key, err := GenerateNodeConfigAboutListeners(&listener)
	require.NoError(t, err)
	cfg.Server.Listeners = data
	cfg.Server.ListenersKey = key

	// run
	Node, err := node.New(cfg)
	require.NoError(t, err)
	testsuite.IsDestroyed(t, cfg)
	go func() {
		err := Node.Main()
		require.NoError(t, err)
	}()
	Node.Wait()
	return Node
}

func testGetNodeListener(t testing.TB, node *node.Node, tag string) *bootstrap.Listener {
	listener, err := node.GetListener(tag)
	require.NoError(t, err)
	addr := listener.Addr()
	return &bootstrap.Listener{
		Mode:    xnet.ModeTLS,
		Network: addr.Network(),
		Address: addr.String(),
	}
}

func testGenerateClient(t testing.TB, node *node.Node) *Client {
	listener := testGetNodeListener(t, node, testInitialNodeListenerTag)
	client, err := ctrl.NewClient(context.Background(), listener, nil, nil)
	require.NoError(t, err)
	return client
}

func TestClient_Send(t *testing.T) {
	Node := testGenerateInitialNode(t)
	client := testGenerateClient(t, Node)

	data := bytes.Buffer{}
	for i := 0; i < 16384; i++ {
		data.Write(convert.Int32ToBytes(int32(i)))
		reply, err := client.send(protocol.TestCommand, data.Bytes())
		require.NoError(t, err)
		require.Equal(t, data.Bytes(), reply)
		data.Reset()
	}

	// clean
	client.Close()
	testsuite.IsDestroyed(t, client)
	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)
}

func TestClient_SendParallel(t *testing.T) {
	Node := testGenerateInitialNode(t)
	client := testGenerateClient(t, Node)

	wg := sync.WaitGroup{}
	send := func() {
		defer wg.Done()
		data := bytes.Buffer{}
		for i := 0; i < 32; i++ {
			data.Write(convert.Int32ToBytes(int32(i)))
			reply, err := client.send(protocol.TestCommand, data.Bytes())
			require.NoError(t, err)
			require.Equal(t, data.Bytes(), reply)
			data.Reset()
		}
	}
	for i := 0; i < 2*protocol.SlotSize; i++ {
		wg.Add(1)
		go send()
	}
	wg.Wait()

	// clean
	client.Close()
	testsuite.IsDestroyed(t, client)
	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)
}

func BenchmarkClient_Send(b *testing.B) {
	Node := testGenerateInitialNode(b)
	client := testGenerateClient(b, Node)

	data := bytes.Buffer{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data.Write(convert.Int32ToBytes(int32(i)))
		reply, err := client.send(protocol.TestCommand, data.Bytes())
		if err != nil {
			b.Fatal(err)
		}
		if !bytes.Equal(data.Bytes(), reply) {
			b.Fatal("reply the different data")
		}
		data.Reset()
	}
	b.StopTimer()

	// clean
	client.Close()
	testsuite.IsDestroyed(b, client)
	Node.Exit(nil)
	testsuite.IsDestroyed(b, Node)
}

func BenchmarkClient_SendParallel(b *testing.B) {
	Node := testGenerateInitialNode(b)
	client := testGenerateClient(b, Node)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		data := bytes.Buffer{}
		i := 0
		for pb.Next() {
			data.Write(convert.Int32ToBytes(int32(i)))
			reply, err := client.send(protocol.TestCommand, data.Bytes())
			if err != nil {
				b.Fatal(err)
			}
			if !bytes.Equal(data.Bytes(), reply) {
				b.Fatal("reply the different data")
			}
			data.Reset()
			i++
		}
	})
	b.StopTimer()

	// clean
	client.Close()
	testsuite.IsDestroyed(b, client)
	Node.Exit(nil)
	testsuite.IsDestroyed(b, Node)
}
