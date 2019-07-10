package controller

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/crypto/aes"
	"project/internal/xnet"
	"project/node"
	"project/testdata"
)

func test_gen_node(t *testing.T, genesis bool) *node.NODE {
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

func Test_client(t *testing.T) {
	NODE := test_gen_node(t, true)
	go func() { _ = NODE.Main() }()
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
	client.Send(0x12, []byte{12, 13})
	/*
		reply, err := client.Send(0x12, []byte{12, 13})
		require.Nil(t, err, err)
		t.Log(reply)
	*/
	// ctrl.Exit(nil)
}
