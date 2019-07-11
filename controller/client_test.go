package controller

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

func test_gen_node(t require.TestingT, genesis bool) *node.NODE {
	reg_aes_key := bytes.Repeat([]byte{0}, aes.BIT256+aes.IV_SIZE)
	c := &node.Config{
		Log_Level: "debug",

		Proxy_Clients:      testdata.Proxy_Clients(t),
		DNS_Clients:        testdata.DNS_Clients(t),
		DNS_Cache_Deadline: 3 * time.Minute,
		Timesync_Clients:   testdata.Timesync(t),
		Timesync_Interval:  15 * time.Minute,

		CTRL_ED25519: testdata.CTRL_ED25519.PublicKey(),
		CTRL_AES_Key: testdata.CTRL_AES_Key,

		Is_Genesis:       genesis,
		Register_AES_Key: reg_aes_key,

		Conn_Limit: 10,
		Listeners:  testdata.Listeners(t),
	}
	// encrypt register info
	register := testdata.Register(t)
	for i := 0; i < len(register); i++ {
		config_enc, err := aes.CBC_Encrypt(register[i].Config,
			reg_aes_key[:aes.BIT256], reg_aes_key[aes.BIT256:])
		require.Nil(t, err, err)
		register[i].Config = config_enc
	}
	c.Register_Bootstraps = register
	n, err := node.New(c)
	require.Nil(t, err, err)
	return n
}

func Test_client_Send(t *testing.T) {
	NODE := test_gen_node(t, true)
	go func() {
		err := NODE.Main()
		require.Nil(t, err, err)
	}()
	NODE.Wait()
	defer NODE.Exit(nil)
	init_ctrl(t)
	config := &client_cfg{
		Node: &bootstrap.Node{
			Mode:    xnet.TLS,
			Network: "tcp",
			Address: "localhost:9950",
		},
	}
	config.TLS_Config.InsecureSkipVerify = true
	client, err := new_client(ctrl, config)
	require.Nil(t, err, err)
	data := bytes.Repeat([]byte{1}, 128)
	reply, err := client.Send(protocol.TEST_MSG, data)
	require.Nil(t, err, err)
	require.Equal(t, data, reply)
	client.Close()
}

func Test_client_Send_parallel(t *testing.T) {
	NODE := test_gen_node(t, true)
	go func() {
		err := NODE.Main()
		require.Nil(t, err, err)
	}()
	NODE.Wait()
	defer NODE.Exit(nil)
	init_ctrl(t)
	config := &client_cfg{
		Node: &bootstrap.Node{
			Mode:    xnet.TLS,
			Network: "tcp",
			Address: "localhost:9950",
		},
	}
	config.TLS_Config.InsecureSkipVerify = true
	client, err := new_client(ctrl, config)
	require.Nil(t, err, err)
	wg := sync.WaitGroup{}
	send := func() {
		data := bytes.NewBuffer(nil)
		for i := 0; i < 1024; i++ {
			data.Write(convert.Int32_Bytes(int32(i)))
			reply, err := client.Send(protocol.TEST_MSG, data.Bytes())
			require.Nil(t, err, err)
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

func Benchmark_client_Send(b *testing.B) {
	NODE := test_gen_node(b, true)
	go func() {
		err := NODE.Main()
		require.Nil(b, err, err)
	}()
	NODE.Wait()
	defer NODE.Exit(nil)
	init_ctrl(b)
	config := &client_cfg{
		Node: &bootstrap.Node{
			Mode:    xnet.TLS,
			Network: "tcp",
			Address: "localhost:9950",
		},
	}
	config.TLS_Config.InsecureSkipVerify = true
	client, err := new_client(ctrl, config)
	require.Nil(b, err, err)
	data := bytes.NewBuffer(nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data.Write(convert.Int32_Bytes(int32(b.N)))
		_, _ = client.Send(protocol.TEST_MSG, data.Bytes())
		// reply, err := client.Send(protocol.TEST_MSG, data.Bytes())
		// require.Nil(b, err, err)
		// require.Equal(b, data.Bytes(), reply)
		data.Reset()
	}
	b.StopTimer()
	client.Close()
}

func Benchmark_client_Send_parallel(b *testing.B) {
	NODE := test_gen_node(b, true)
	go func() {
		err := NODE.Main()
		require.Nil(b, err, err)
	}()
	NODE.Wait()
	defer NODE.Exit(nil)
	init_ctrl(b)
	config := &client_cfg{
		Node: &bootstrap.Node{
			Mode:    xnet.TLS,
			Network: "tcp",
			Address: "localhost:9950",
		},
	}
	config.TLS_Config.InsecureSkipVerify = true
	client, err := new_client(ctrl, config)
	require.Nil(b, err, err)
	n_one := b.N / runtime.NumCPU()
	wg := sync.WaitGroup{}
	send := func() {
		data := bytes.NewBuffer(nil)
		for i := 0; i < n_one; i++ {
			data.Write(convert.Int32_Bytes(int32(i)))
			_, _ = client.Send(protocol.TEST_MSG, data.Bytes())
			// reply, err := client.Send(protocol.TEST_MSG, data.Bytes())
			// require.Nil(b, err, err)
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
