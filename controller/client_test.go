package controller

/*
import (
	"bytes"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/protocol"
	"project/internal/xnet"
	"project/node"
	"project/testdata"
)

func testGenerateNode(t require.TestingT, genesis bool) *node.NODE {
	regAESKey := bytes.Repeat([]byte{0}, aes.Bit256+aes.IVSize)
	c := &node.Config{
		Log_Level: "debug",

		Proxy_Clients:      testdata.ProxyClients(t),
		DNS_Clients:        testdata.DNSServers(t),
		DNS_Cache_Deadline: 3 * time.Minute,
		Timesync_Clients:   testdata.TimeSyncerConfigs(t),
		Timesync_Interval:  15 * time.Minute,

		CTRL_ED25519: testdata.CtrlED25519.PublicKey(),
		CTRL_AES_Key: testdata.CtrlAESKey,

		Is_Genesis:       genesis,
		Register_AES_Key: regAESKey,

		Conn_Limit: 10,
		Listeners:  testdata.Listeners(t),
	}
	c.Debug.Skip_Timesync = true
	// encrypt register info
	register := testdata.Register(t)
	for i := 0; i < len(register); i++ {
		configEnc, err := aes.CBCEncrypt(register[i].Config,
			regAESKey[:aes.Bit256], regAESKey[aes.Bit256:])
		require.NoError(t, err)
		register[i].Config = configEnc
	}
	c.Register_Bootstraps = register
	n, err := node.New(c)
	require.NoError(t, err)
	return n
}

func TestClient_Send(t *testing.T) {
	NODE := testGenerateNode(t, true)
	go func() {
		err := NODE.Main()
		require.NoError(t, err)
	}()
	NODE.Wait()
	defer NODE.Exit(nil)
	initCtrl(t)
	config := &clientCfg{
		Node: &bootstrap.Node{
			Mode:    xnet.TLS,
			Network: "tcp",
			Address: "localhost:9950",
		},
	}
	config.TLSConfig.InsecureSkipVerify = true
	client, err := newClient(ctrl, config)
	require.NoError(t, err)
	data := bytes.Repeat([]byte{1}, 128)
	reply, err := client.Send(protocol.TestMessage, data)
	require.NoError(t, err)
	require.Equal(t, data, reply)
	client.Close()
}

func TestClient_SendParallel(t *testing.T) {
	NODE := testGenerateNode(t, true)
	go func() {
		err := NODE.Main()
		require.NoError(t, err)
	}()
	NODE.Wait()
	defer NODE.Exit(nil)
	initCtrl(t)
	config := &clientCfg{
		Node: &bootstrap.Node{
			Mode:    xnet.TLS,
			Network: "tcp",
			Address: "localhost:9950",
		},
	}
	config.TLSConfig.InsecureSkipVerify = true
	client, err := newClient(ctrl, config)
	require.NoError(t, err)
	wg := sync.WaitGroup{}
	send := func() {
		data := bytes.NewBuffer(nil)
		for i := 0; i < 1024; i++ {
			data.Write(convert.Int32ToBytes(int32(i)))
			reply, err := client.Send(protocol.TestMessage, data.Bytes())
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
	NODE := testGenerateNode(b, true)
	go func() {
		err := NODE.Main()
		require.NoError(b, err)
	}()
	NODE.Wait()
	defer NODE.Exit(nil)
	initCtrl(b)
	config := &clientCfg{
		Node: &bootstrap.Node{
			Mode:    xnet.TLS,
			Network: "tcp",
			Address: "localhost:9950",
		},
	}
	config.TLSConfig.InsecureSkipVerify = true
	client, err := newClient(ctrl, config)
	require.NoError(b, err)
	data := bytes.NewBuffer(nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data.Write(convert.Int32ToBytes(int32(b.N)))
		_, _ = client.Send(protocol.TestMessage, data.Bytes())
		// reply, err := client.Send(protocol.TestMessage, data.Bytes())
		// require.NoError(b, err)
		// require.Equal(b, data.Bytes(), reply)
		data.Reset()
	}
	b.StopTimer()
	client.Close()
}

func BenchmarkClient_SendParallel(b *testing.B) {
	NODE := testGenerateNode(b, true)
	go func() {
		err := NODE.Main()
		require.NoError(b, err)
	}()
	NODE.Wait()
	defer NODE.Exit(nil)
	initCtrl(b)
	config := &clientCfg{
		Node: &bootstrap.Node{
			Mode:    xnet.TLS,
			Network: "tcp",
			Address: "localhost:9950",
		},
	}
	config.TLSConfig.InsecureSkipVerify = true
	client, err := newClient(ctrl, config)
	require.NoError(b, err)
	nOnce := b.N / runtime.NumCPU()
	wg := sync.WaitGroup{}
	send := func() {
		data := bytes.NewBuffer(nil)
		for i := 0; i < nOnce; i++ {
			data.Write(convert.Int32ToBytes(int32(i)))
			_, _ = client.Send(protocol.TestMessage, data.Bytes())
			// reply, err := client.Send(protocol.TestMessage, data.Bytes())
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
*/
