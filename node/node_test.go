package node

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/testdata"
)

const (
	proxy_socks5 = "test_socks5_client"
	proxy_http   = "test_http_proxy_client"
)

func Test_Node(t *testing.T) {
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

	// err = node.Main()
	// require.Nil(t, err, err)
}

func test_generate_config(t *testing.T) *Config {
	c := &Config{
		Log_level:          "debug",
		Proxy_Clients:      testdata.Proxy_Clients(t),
		DNS_Clients:        testdata.DNS_Clients(t),
		DNS_Cache_Deadline: 3 * time.Minute,
		Timesync_Clients:   testdata.Timesync_Client(t),
		Timesync_Interval:  15 * time.Minute,
	}
	return c
}
