package proxy

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

const (
	tagSocks5 = "test_socks5_client"
	tagHTTP   = "test_http_proxy_client"
)

func TestPool(t *testing.T) {
	config, err := ioutil.ReadFile("testdata/socks5.toml")
	require.NoError(t, err)
	s5c := &Chain{
		Mode:   Socks5,
		Config: config,
	}
	config, err = ioutil.ReadFile("testdata/http.txt")
	require.NoError(t, err)
	hp := &Chain{
		Mode:   HTTP,
		Config: config,
	}
	clients := make(map[string]*Chain)
	clients[tagSocks5] = s5c
	clients[tagHTTP] = hp
	pool, err := NewPool(clients)
	require.NoError(t, err)
	// add empty tag
	err = pool.Add("", s5c)
	require.Equal(t, ErrEmptyTag, err)
	// add client with reserve tag
	err = pool.Add("direct", s5c)
	require.Equal(t, ErrReserveTag, err)
	// add unknown mode
	err = pool.Add("foo", &Chain{Mode: "foo"})
	require.Error(t, err)
	require.Equal(t, "unknown mode: foo", err.Error())
	// add exist
	err = pool.Add(tagSocks5, s5c)
	require.Error(t, err)
	require.Equal(t, "proxy client: "+tagSocks5+" already exists", err.Error())
	// get
	pc, err := pool.Get(tagSocks5)
	require.NoError(t, err)
	require.NotNil(t, pc)
	pc, err = pool.Get(tagHTTP)
	require.NoError(t, err)
	require.NotNil(t, pc)
	// get direct
	pc, err = pool.Get("")
	require.NoError(t, err)
	require.NotNil(t, pc)
	pc, err = pool.Get("direct")
	require.NoError(t, err)
	require.NotNil(t, pc)
	// get doesn't exist
	pc, err = pool.Get("doesn't exist")
	require.Error(t, err)
	require.Nil(t, pc)
	// get all clients
	for tag, client := range pool.Clients() {
		t.Logf("tag: %s mode: %s info: %s", tag, client.Mode, client.Info())
	}
	// delete
	err = pool.Delete(tagHTTP)
	require.NoError(t, err)
	// delete doesn't exist
	err = pool.Delete(tagHTTP)
	require.Error(t, err)
	require.Equal(t, "proxy client: "+tagHTTP+" doesn't exist", err.Error())
	// delete client with empty tag
	err = pool.Delete("")
	require.Equal(t, ErrEmptyTag, err)
	// delete reserve client
	err = pool.Delete("direct")
	require.Equal(t, ErrReserveClient, err)
	testutil.IsDestroyed(t, pool, 1)
}
