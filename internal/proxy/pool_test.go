package proxy

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

const (
	testTagSocks5  = "test_socks5"
	testTagSocks4a = "test_socks4a"
	testTagSocks4  = "test_socks4"
	testTagHTTP    = "test_http"
	testTagHTTPS   = "test_https"
	testTagChain   = "test_chain"
	testTagBalance = "test_balance"
)

func testGeneratePool(t *testing.T) *Pool {
	options, err := ioutil.ReadFile("socks/testdata/socks5_client.toml")
	require.NoError(t, err)
	socks5Client := &Client{
		Tag:     testTagSocks5,
		Mode:    ModeSocks5,
		Network: "tcp",
		Address: "localhost:1080",
		Options: string(options),
	}
	options, err = ioutil.ReadFile("socks/testdata/socks4a_client.toml")
	require.NoError(t, err)
	socks4aClient := &Client{
		Tag:     testTagSocks4a,
		Mode:    ModeSocks4a,
		Network: "tcp",
		Address: "localhost:1080",
		Options: string(options),
	}
	options, err = ioutil.ReadFile("socks/testdata/socks4_client.toml")
	require.NoError(t, err)
	socks4Client := &Client{
		Tag:     testTagSocks4,
		Mode:    ModeSocks4,
		Network: "tcp",
		Address: "localhost:1080",
		Options: string(options),
	}
	options, err = ioutil.ReadFile("http/testdata/http.toml")
	require.NoError(t, err)
	httpClient := &Client{
		Tag:     testTagHTTP,
		Mode:    ModeHTTP,
		Network: "tcp",
		Address: "localhost:1080",
		Options: string(options),
	}
	options, err = ioutil.ReadFile("http/testdata/https.toml")
	require.NoError(t, err)
	httpsClient := &Client{
		Tag:     testTagHTTPS,
		Mode:    ModeHTTP,
		Network: "tcp",
		Address: "localhost:1080",
		Options: string(options),
	}
	options, err = ioutil.ReadFile("testdata/chain.toml")
	require.NoError(t, err)
	chain := &Client{
		Tag:     testTagChain,
		Mode:    ModeChain,
		Options: string(options),
	}
	options, err = ioutil.ReadFile("testdata/balance.toml")
	require.NoError(t, err)
	balance := &Client{
		Tag:     testTagBalance,
		Mode:    ModeBalance,
		Options: string(options),
	}
	pool := NewPool()
	require.NoError(t, pool.Add(socks5Client))
	require.NoError(t, pool.Add(socks4aClient))
	require.NoError(t, pool.Add(socks4Client))
	require.NoError(t, pool.Add(httpClient))
	require.NoError(t, pool.Add(httpsClient))
	require.NoError(t, pool.Add(chain))
	require.NoError(t, pool.Add(balance))
	return pool
}

func TestPool_Add(t *testing.T) {
	pool := testGeneratePool(t)

	t.Run("with empty tag", func(t *testing.T) {
		err := pool.Add(new(Client))
		require.EqualError(t, err, "empty proxy client tag")
	})

	t.Run("with reserve tag", func(t *testing.T) {
		client := &Client{Tag: ModeDirect}
		err := pool.Add(client)
		require.EqualError(t, err, "direct is the reserve proxy client")
	})

	t.Run("with unknown mode", func(t *testing.T) {
		client := &Client{
			Tag:  "foo",
			Mode: "foo mode",
		}
		err := pool.Add(client)
		require.EqualError(t, err, "unknown mode: foo mode")
	})

	t.Run("exist", func(t *testing.T) {
		options, err := ioutil.ReadFile("socks/testdata/socks5_client.toml")
		require.NoError(t, err)
		socks5Client := &Client{
			Tag:     testTagSocks5,
			Mode:    ModeSocks5,
			Network: "tcp",
			Address: "localhost:1080",
			Options: string(options),
		}
		err = pool.Add(socks5Client)
		require.Errorf(t, err, "proxy client %s already exists", testTagSocks5)
	})

	t.Run("failed to add", func(t *testing.T) {
		t.Run("socks client with invalid toml data", func(t *testing.T) {
			err := pool.Add(&Client{
				Tag:     "invalid socks5",
				Mode:    ModeSocks5,
				Options: "socks4 = foo",
			})
			require.Error(t, err)
		})

		t.Run("socks client with invalid options", func(t *testing.T) {
			err := pool.Add(&Client{
				Tag:     "invalid socks5",
				Mode:    ModeSocks5,
				Network: "foo network",
			})
			require.Error(t, err)
		})

		t.Run("http proxy client with invalid toml data", func(t *testing.T) {
			err := pool.Add(&Client{
				Tag:     "invalid http",
				Mode:    ModeHTTP,
				Options: "https = foo",
			})
			require.Error(t, err)
		})

		t.Run("http proxy client with invalid options", func(t *testing.T) {
			err := pool.Add(&Client{
				Tag:     "invalid http",
				Mode:    ModeHTTP,
				Network: "foo network",
			})
			require.Error(t, err)
		})

		t.Run("proxy chain with invalid toml data", func(t *testing.T) {
			err := pool.Add(&Client{
				Tag:     "invalid proxy chain",
				Mode:    ModeChain,
				Options: "tag====foo data",
			})
			require.Error(t, err)
		})

		t.Run("proxy chain with doesn't exist client", func(t *testing.T) {
			err := pool.Add(&Client{
				Tag:     "invalid proxy chain",
				Mode:    ModeChain,
				Options: `tags = ["foo_client"]`,
			})
			require.Error(t, err)
		})

		t.Run("proxy chain with empty clients", func(t *testing.T) {
			err := pool.Add(&Client{
				Tag:  "invalid proxy chain",
				Mode: ModeChain,
			})
			require.Error(t, err)
		})

		t.Run("balance with invalid toml data", func(t *testing.T) {
			err := pool.Add(&Client{
				Tag:     "invalid balance",
				Mode:    ModeBalance,
				Options: "tag====foo data",
			})
			require.Error(t, err)
		})

		t.Run("balance with doesn't exist client", func(t *testing.T) {
			err := pool.Add(&Client{
				Tag:     "invalid balance",
				Mode:    ModeBalance,
				Options: `tags = ["foo_client"]`,
			})
			require.Error(t, err)
		})

		t.Run("balance with empty clients", func(t *testing.T) {
			err := pool.Add(&Client{
				Tag:  "invalid balance",
				Mode: ModeBalance,
			})
			require.Error(t, err)
		})
	})

	testsuite.IsDestroyed(t, pool)
}

func TestPool_Get(t *testing.T) {
	pool := testGeneratePool(t)

	t.Run("basic", func(t *testing.T) {
		pc, err := pool.Get(testTagSocks5)
		require.NoError(t, err)
		require.NotNil(t, pc)

		pc, err = pool.Get(testTagSocks4a)
		require.NoError(t, err)
		require.NotNil(t, pc)

		pc, err = pool.Get(testTagSocks4)
		require.NoError(t, err)
		require.NotNil(t, pc)

		pc, err = pool.Get(testTagHTTP)
		require.NoError(t, err)
		require.NotNil(t, pc)

		pc, err = pool.Get(testTagHTTPS)
		require.NoError(t, err)
		require.NotNil(t, pc)
	})

	t.Run("direct", func(t *testing.T) {
		pc, err := pool.Get("")
		require.NoError(t, err)
		require.NotNil(t, pc)

		pc, err = pool.Get(ModeDirect)
		require.NoError(t, err)
		require.NotNil(t, pc)
	})

	t.Run("doesn't exist", func(t *testing.T) {
		pc, err := pool.Get("foo")
		require.EqualError(t, err, "proxy client foo doesn't exist")
		require.Nil(t, pc)
	})

	t.Run("all", func(t *testing.T) {
		for tag, client := range pool.Clients() {
			t.Logf("tag: %s mode: %s info: %s\n", tag, client.Mode, client.Info())
		}
	})

	testsuite.IsDestroyed(t, pool)
}

func TestPool_Delete(t *testing.T) {
	pool := testGeneratePool(t)

	t.Run("http", func(t *testing.T) {
		err := pool.Delete(testTagHTTP)
		require.NoError(t, err)
	})

	t.Run("doesn't exist", func(t *testing.T) {
		err := pool.Delete("foo")
		require.EqualError(t, err, "proxy client foo doesn't exist")
	})

	t.Run("empty tag", func(t *testing.T) {
		err := pool.Delete("")
		require.EqualError(t, err, "empty proxy client tag")
	})

	t.Run("direct", func(t *testing.T) {
		err := pool.Delete("direct")
		require.EqualError(t, err, "direct is the reserve proxy client")
	})

	testsuite.IsDestroyed(t, pool)
}
