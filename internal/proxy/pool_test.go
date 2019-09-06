package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	tagSocks5 = "test_socks5_client"
	tagHTTP   = "test_http_proxy_client"
)

func TestPool(t *testing.T) {
	clients := make(map[string]*Client)
	// create socks5 client config(s5c)
	s5c := `
        [[Clients]]
          Address = "localhost:0"
          Network = "tcp"
          Password = "123456"
          Username = "admin"
        
        [[Clients]]
          Address = "localhost:0"
          Network = "tcp"
          Password = "123456"
          Username = "admin"
`
	clients[tagSocks5] = &Client{
		Mode:   Socks5,
		Config: s5c,
	}
	clients[tagHTTP] = &Client{
		Mode:   HTTP,
		Config: "http://admin:123456@localhost:0",
	}
	// make
	pool, err := NewPool(clients)
	require.NoError(t, err)
	// get
	pc, err := pool.Get(tagSocks5)
	require.NoError(t, err)
	require.NotNil(t, pc)
	// get nil
	pc, err = pool.Get("")
	require.NoError(t, err)
	require.Nil(t, pc)
	// get failed
	pc, err = pool.Get("doesn't exist")
	require.Error(t, err)
	require.Nil(t, pc)
	// list
	for k := range pool.Clients() {
		t.Log("client:", k)
	}
	// add reserve
	err = pool.Add("", nil)
	require.Equal(t, ErrReserveProxy, err)
	// add exist
	err = pool.Add(tagSocks5, &Client{
		Mode:   Socks5,
		Config: s5c},
	)
	require.Error(t, err)
	// unknown mode
	err = pool.Add("unknown mode", &Client{
		Mode:   "unknown mode",
		Config: s5c},
	)
	require.Equal(t, ErrUnknownMode, err)
	// delete
	err = pool.Delete(tagHTTP)
	require.NoError(t, err)
	// delete reserve
	err = pool.Delete("")
	require.Equal(t, ErrReserveProxy, err)
	// delete doesn't exist
	err = pool.Delete(tagHTTP)
	require.Error(t, err)
	// New failed == add failed
	clients[tagSocks5] = &Client{
		Mode:   Socks5,
		Config: "invalid toml config"}
	pool, err = NewPool(clients)
	require.Error(t, err)
	require.Nil(t, pool)
}
