package proxyclient

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/proxy"
	"project/internal/proxy/socks5"
)

const (
	proxy_socks5 = "test_socks5_client"
	proxy_http   = "test_http_proxy_client"
)

func Test_PROXY(t *testing.T) {
	clients := make(map[string]*Client)
	// create socks5 client config(s5c)
	s5cc := &socks5.Config{
		Network:  "tcp",
		Address:  "localhost:0",
		Username: "admin",
		Password: "123456",
	}
	clients[proxy_socks5] = &Client{
		Mode:   proxy.SOCKS5,
		Config: []*socks5.Config{s5cc},
	}
	clients[proxy_http] = &Client{
		Mode:   proxy.HTTP,
		Config: "http://admin:123456@localhost:0",
	}
	// make
	PROXY, err := New(clients)
	require.Nil(t, err, err)
	// get
	pc, err := PROXY.Get(proxy_socks5)
	require.Nil(t, err, err)
	require.NotNil(t, pc)
	// get nil
	pc, err = PROXY.Get("")
	require.Nil(t, err, err)
	require.Nil(t, pc)
	// get failed
	pc, err = PROXY.Get("not exists")
	require.NotNil(t, err, err)
	require.Nil(t, pc)
	// list
	for k := range PROXY.Clients() {
		t.Log("client:", k)
	}
	// add reserve
	err = PROXY.Add("", nil)
	require.Equal(t, err, ERR_RESERVE_PROXY, err)
	// add exist
	err = PROXY.Add(proxy_socks5, &Client{
		Mode:   proxy.SOCKS5,
		Config: []*socks5.Config{s5cc}},
	)
	require.NotNil(t, err, err)
	// delete
	err = PROXY.Delete(proxy_http)
	require.Nil(t, err, err)
	// delete reserve
	err = PROXY.Delete("")
	require.Equal(t, err, ERR_RESERVE_PROXY, err)
	// delete doesn't exist
	err = PROXY.Delete(proxy_http)
	require.NotNil(t, err, err)
	PROXY.Destroy()
	// New failed == add failed
	clients[proxy_socks5] = &Client{
		Mode:   proxy.SOCKS5,
		Config: s5cc}
	PROXY, err = New(clients)
	require.NotNil(t, err)
	require.Nil(t, PROXY)
}
