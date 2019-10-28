package proxy

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

func TestPool(t *testing.T) {
	options, err := ioutil.ReadFile("testdata/socks5_opts.toml")
	require.NoError(t, err)
	socksClient := &Client{
		Mode:    ModeSocks,
		Network: "tcp",
		Address: "localhost:1080",
		Options: string(options),
	}
	options, err = ioutil.ReadFile("testdata/http_opts.toml")
	require.NoError(t, err)
	httpClient := &Client{
		Mode:    ModeHTTP,
		Network: "tcp",
		Address: "localhost:1080",
		Options: string(options),
	}
	options, err = ioutil.ReadFile("testdata/chain.toml")
	require.NoError(t, err)
	chain := &Client{
		Mode:    ModeChain,
		Options: string(options),
	}
	options, err = ioutil.ReadFile("testdata/balance.toml")
	require.NoError(t, err)
	balance := &Client{
		Mode:    ModeBalance,
		Options: string(options),
	}
	const (
		tagSocks   = "test_socks"
		tagHTTP    = "test_http"
		tagChain   = "test_chain"
		tagBalance = "test_balance"
	)
	clients := make(map[string]*Client)
	clients[tagSocks] = socksClient
	clients[tagHTTP] = httpClient
	clients[tagChain] = chain
	clients[tagBalance] = balance
	pool, err := NewPool(clients)
	require.NoError(t, err)
	// add client with empty tag
	err = pool.Add("", socksClient)
	require.Errorf(t, err, "empty proxy client tag")
	// add client with reserve tag
	err = pool.Add("direct", socksClient)
	require.Errorf(t, err, "direct is the reserve proxy client")
	// add unknown mode
	err = pool.Add("foo", &Client{Mode: "foo mode"})
	require.Errorf(t, err, "unknown mode: foo mode")
	// add exist
	err = pool.Add(tagSocks, socksClient)
	require.Errorf(t, err, "proxy client %s already exists", tagSocks)
	// get
	pc, err := pool.Get(tagSocks)
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
	pc, err = pool.Get("foo")
	require.Errorf(t, err, "proxy client foo doesn't exist")
	require.Nil(t, pc)
	// get all clients info
	for tag, client := range pool.Clients() {
		t.Logf("tag: %s mode: %s info: %s", tag, client.Mode, client.Info())
	}
	// delete
	err = pool.Delete(tagHTTP)
	require.NoError(t, err)
	// delete doesn't exist
	err = pool.Delete(tagHTTP)
	require.Errorf(t, err, "proxy client %s doesn't exist", tagHTTP)
	// delete client with empty tag
	err = pool.Delete("")
	require.Errorf(t, err, "empty proxy client tag")
	// delete direct
	err = pool.Delete("direct")
	require.Errorf(t, err, "direct is the reserve proxy client")
	testutil.IsDestroyed(t, pool)
}
