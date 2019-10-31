package proxy

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestPool(t *testing.T) {
	const (
		tagSocks   = "test_socks"
		tagHTTP    = "test_http"
		tagChain   = "test_chain"
		tagBalance = "test_balance"
	)
	options, err := ioutil.ReadFile("testdata/socks5_opts.toml")
	require.NoError(t, err)
	socksClient := &Client{
		Tag:     tagSocks,
		Mode:    ModeSocks,
		Network: "tcp",
		Address: "localhost:1080",
		Options: string(options),
	}
	options, err = ioutil.ReadFile("testdata/http_opts.toml")
	require.NoError(t, err)
	httpClient := &Client{
		Tag:     tagHTTP,
		Mode:    ModeHTTP,
		Network: "tcp",
		Address: "localhost:1080",
		Options: string(options),
	}
	options, err = ioutil.ReadFile("testdata/chain.toml")
	require.NoError(t, err)
	chain := &Client{
		Tag:     tagChain,
		Mode:    ModeChain,
		Options: string(options),
	}
	options, err = ioutil.ReadFile("testdata/balance.toml")
	require.NoError(t, err)
	balance := &Client{
		Tag:     tagBalance,
		Mode:    ModeBalance,
		Options: string(options),
	}
	pool := NewPool()
	require.NoError(t, pool.Add(socksClient))
	require.NoError(t, pool.Add(httpClient))
	require.NoError(t, pool.Add(chain))
	require.NoError(t, pool.Add(balance))
	// add client with empty tag
	testClient := &Client{}
	err = pool.Add(testClient)
	require.Errorf(t, err, "empty proxy client tag")
	// add client with reserve tag
	testClient.Tag = "direct"
	err = pool.Add(testClient)
	require.Errorf(t, err, "direct is the reserve proxy client")
	// add unknown mode
	testClient.Tag = "foo"
	testClient.Mode = "foo mode"
	err = pool.Add(testClient)
	require.Errorf(t, err, "unknown mode: foo mode")
	// add exist
	err = pool.Add(socksClient)
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
	testsuite.IsDestroyed(t, pool)
}

func TestPool_Add(t *testing.T) {
	pool := NewPool()
	// add socks client with invalid toml data
	err := pool.Add(&Client{
		Tag:     "invalid socks5",
		Mode:    ModeSocks,
		Options: "socks4 = foo",
	})
	require.Error(t, err)

	// add socks client with invalid options
	err = pool.Add(&Client{
		Tag:     "invalid socks5",
		Mode:    ModeSocks,
		Network: "foo network",
	})
	require.Error(t, err)

	// add http proxy client with invalid toml data
	err = pool.Add(&Client{
		Tag:     "invalid http",
		Mode:    ModeHTTP,
		Options: "https = foo",
	})
	require.Error(t, err)

	// add http proxy client with invalid options
	err = pool.Add(&Client{
		Tag:     "invalid http",
		Mode:    ModeHTTP,
		Network: "foo network",
	})
	require.Error(t, err)

	// add proxy chain with invalid toml data
	err = pool.Add(&Client{
		Tag:     "invalid proxy chain",
		Mode:    ModeChain,
		Options: "tag====foo data",
	})
	require.Error(t, err)

	// add proxy chain with doesn't exist client
	err = pool.Add(&Client{
		Tag:     "invalid proxy chain",
		Mode:    ModeChain,
		Options: `tags = ["foo_client"]`,
	})
	require.Error(t, err)

	// add proxy chain with no clients
	err = pool.Add(&Client{
		Tag:  "invalid proxy chain",
		Mode: ModeChain,
	})
	require.Error(t, err)

	// add balance with invalid toml data
	err = pool.Add(&Client{
		Tag:     "invalid balance",
		Mode:    ModeBalance,
		Options: "tag====foo data",
	})
	require.Error(t, err)

	// add balance with doesn't exist client
	err = pool.Add(&Client{
		Tag:     "invalid balance",
		Mode:    ModeBalance,
		Options: `tags = ["foo_client"]`,
	})
	require.Error(t, err)

	// add balance with no clients
	err = pool.Add(&Client{
		Tag:  "invalid balance",
		Mode: ModeBalance,
	})
	require.Error(t, err)
}
