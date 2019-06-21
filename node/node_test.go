package node

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/testdata"
)

func Test_Node(t *testing.T) {
	config := test_generate_config(t)
	node, err := New(config)
	require.Nil(t, err, err)
	err = node.Main()
	require.Nil(t, err, err)
}

func Test_Check_Config(t *testing.T) {
	config := test_generate_config(t)
	err := config.Check()
	require.Nil(t, err, err)
	node, err := New(config)
	require.Nil(t, err, err)
	for k := range node.global.proxy.Clients() {
		t.Log("proxy client:", k)
	}
	for k := range node.global.dns.Clients() {
		t.Log("dns client:", k)
	}
	for k := range node.global.timesync.Clients() {
		t.Log("timesync client:", k)
	}
}

func test_generate_config(t *testing.T) *Config {
	reg_aes_key := bytes.Repeat([]byte{0}, aes.BIT256)
	reg_aes_iv := bytes.Repeat([]byte{0}, aes.IV_SIZE)
	c := &Config{
		Log_level:          "debug",
		Proxy_Clients:      testdata.Proxy_Clients(t),
		DNS_Clients:        testdata.DNS_Clients(t),
		DNS_Cache_Deadline: 3 * time.Minute,
		Timesync_Clients:   testdata.Timesync_Clients(t),
		Timesync_Interval:  15 * time.Minute,
		Register_AES_Key:   reg_aes_key,
		Register_AES_IV:    reg_aes_iv,
	}
	register := testdata.Register(t)
	for i := 0; i < len(register); i++ {
		config_enc, err := aes.CBC_Encrypt(register[i].Config,
			reg_aes_key, reg_aes_iv)
		require.Nil(t, err, err)
		register[i].Config = config_enc
	}
	c.Register_Config = register
	return c
}
