package controller

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
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
	test_data := bytes.Repeat([]byte{0}, 128)
	reply, err := client.Send(protocol.TEST_MSG, test_data)
	require.Nil(t, err, err)
	require.Equal(t, test_data, reply)
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
	test_data := bytes.Repeat([]byte{0}, 128)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Send(protocol.TEST_MSG, test_data)

		// reply, err := client.Send(protocol.TEST_MSG, test_data)
		// require.Nil(b, err, err)
		// require.Equal(b, test_data, reply)
	}
	b.StopTimer()
	client.Close()
}
