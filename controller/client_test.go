package controller

import (
	"bytes"
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
	"project/internal/xnet"
	"project/node"
)

func testGenerateNodeConfig(t require.TestingT, genesis bool) *node.Config {
	cfg := node.Config{}

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

func testGenerateNode(t require.TestingT, genesis bool) *node.Node {
	cfg := testGenerateNodeConfig(t, genesis)
	NODE, err := node.New(cfg)
	require.NoError(t, err)
	go func() {
		err := NODE.Main()
		require.NoError(t, err)
	}()
	NODE.TestWait()

	// generate certificate
	pks := ctrl.global.GetSelfCA()
	opts := cert.Options{DNSNames: []string{"localhost"}}
	caCert := pks[0].Certificate
	caKey := pks[0].PrivateKey
	kp, err := cert.Generate(caCert, caKey, &opts)
	require.NoError(t, err)

	// generate listener config
	listener := messages.Listener{
		Tag:     "test_tls_listener",
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
	require.NoError(t, NODE.AddListener(&listener))
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
