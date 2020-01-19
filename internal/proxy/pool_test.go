package proxy

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

var testClientTags = []string{
	"socks5",
	"socks4a",
	"socks4",
	"http",
	"https",
	"chain",
	"balance",
}

func testGeneratePool(t *testing.T) *Pool {
	pool := NewPool()
	for i, filename := range []string{
		"socks/testdata/socks5_client.toml",
		"socks/testdata/socks4a_client.toml",
		"socks/testdata/socks4_client.toml",
		"http/testdata/http_client.toml",
		"http/testdata/https_client.toml",
		"testdata/chain.toml",
		"testdata/balance.toml",
	} {
		opts, err := ioutil.ReadFile(filename)
		require.NoError(t, err)
		client := &Client{
			Tag:     testClientTags[i],
			Mode:    testClientTags[i],
			Network: "tcp",
			Address: "localhost:1080",
			Options: string(opts),
		}
		err = pool.Add(client)
		require.NoError(t, err)
	}
	return pool
}

func TestPool_Add(t *testing.T) {
	pool := testGeneratePool(t)

	t.Run("with empty tag", func(t *testing.T) {
		err := pool.add(new(Client))
		require.EqualError(t, err, "empty proxy client tag")
	})

	t.Run("with reserve tag", func(t *testing.T) {
		client := &Client{Tag: ModeDirect}
		err := pool.add(client)
		require.EqualError(t, err, "direct is the reserve proxy client tag")
	})

	t.Run("with unknown mode", func(t *testing.T) {
		client := &Client{
			Tag:  "foo",
			Mode: "foo mode",
		}
		err := pool.add(client)
		require.EqualError(t, err, "unknown mode: foo mode")
	})

	t.Run("exist", func(t *testing.T) {
		opts, err := ioutil.ReadFile("socks/testdata/socks5_client.toml")
		require.NoError(t, err)
		client := &Client{
			Tag:     ModeSocks5, // same tag
			Mode:    ModeSocks5,
			Network: "tcp",
			Address: "localhost:1080",
			Options: string(opts),
		}
		err = pool.Add(client)
		require.EqualError(t, err, "failed to add proxy client socks5: already exists")
	})

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

	testsuite.IsDestroyed(t, pool)
}

func TestPool_Get(t *testing.T) {
	pool := testGeneratePool(t)

	t.Run("basic", func(t *testing.T) {
		for _, tag := range testClientTags {
			pc, err := pool.Get(tag)
			require.NoError(t, err)
			require.NotNil(t, pc)
		}
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

	t.Run("print all", func(t *testing.T) {
		for tag, client := range pool.Clients() {
			t.Logf("tag: %s mode: %s info: %s\n", tag, client.Mode, client.Info())
		}
	})

	testsuite.IsDestroyed(t, pool)
}

func TestPool_Delete(t *testing.T) {
	pool := testGeneratePool(t)

	t.Run("basic", func(t *testing.T) {
		for _, tag := range testClientTags {
			err := pool.Delete(tag)
			require.NoError(t, err)
		}
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
