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
	"project/internal/messages"
	"project/internal/options"
	"project/internal/protocol"
	"project/internal/testsuite"
	"project/internal/xnet"
	"project/node"
	"project/testdata"
)

const testListenerTag = "test_tls"

func testGenerateNodeConfig(tb testing.TB) *node.Config {
	cfg := node.Config{}

	cfg.Debug.SkipSynchronizeTime = true
	cfg.Debug.Broadcast = make(chan []byte, 4)
	cfg.Debug.Send = make(chan []byte, 4)

	cfg.Logger.Level = "debug"
	cfg.Logger.Writer = os.Stdout

	cfg.Global.DNSCacheExpire = 3 * time.Minute
	cfg.Global.TimeSyncInterval = 1 * time.Minute
	cfg.Global.Certificates = testdata.Certificates(tb)
	cfg.Global.ProxyClients = testdata.ProxyClients(tb)
	cfg.Global.DNSServers = testdata.DNSServers()
	cfg.Global.TimeSyncerClients = testdata.TimeSyncerClients(tb)

	cfg.Client.ProxyTag = "balance"
	cfg.Client.Timeout = 15 * time.Second

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

func testGenerateNode(t testing.TB) *node.Node {
	cfg := testGenerateNodeConfig(t)

	NODE, err := node.New(cfg)
	require.NoError(t, err)

	// generate certificate
	pks := ctrl.global.GetSelfCA()
	opts := cert.Options{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	caCert := pks[0].Certificate
	caKey := pks[0].PrivateKey
	kp, err := cert.Generate(caCert, caKey, &opts)
	require.NoError(t, err)

	// generate listener config
	listener := messages.Listener{
		Tag:     testListenerTag,
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "localhost:0",
	}
	c, k := kp.EncodeToPEM()
	listener.TLSConfig.Certificates = []options.X509KeyPair{
		{
			Cert: string(c),
			Key:  string(k),
		},
	}

	go func() {
		err := NODE.Main()
		require.NoError(t, err)
	}()
	NODE.Wait()
	require.NoError(t, NODE.AddListener(&listener))
	return NODE
}

func testGenerateClient(tb testing.TB, node *node.Node) *client {
	listener, err := node.GetListener(testListenerTag)
	require.NoError(tb, err)
	n := &bootstrap.Node{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: listener.Addr().String(),
	}
	client, err := newClient(context.Background(), ctrl, n, nil, nil)
	require.NoError(tb, err)
	return client
}

func TestClient_Send(t *testing.T) {
	testInitCtrl(t)
	NODE := testGenerateNode(t)
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
	testInitCtrl(t)
	NODE := testGenerateNode(t)
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
	testInitCtrl(b)
	NODE := testGenerateNode(b)
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
	testInitCtrl(b)
	NODE := testGenerateNode(b)
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
