package proxy

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
	"project/internal/testsuite/testcert"
)

var testClientTags = [...]string{
	"socks5",
	"socks4a",
	"socks4",
	"http",
	"https",
	"chain",
	"balance",
}

const testReserveClientNum = 2

var testClientNum = testReserveClientNum + len(testClientTags)

func testGeneratePool(t *testing.T) *Pool {
	pool := NewPool(testcert.CertPool(t))
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
		const errStr = "failed to add proxy client socks5: already exists"
		err = pool.Add(client)
		require.EqualError(t, err, errStr)
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

	clients := pool.Clients()
	require.Len(t, clients, testClientNum)

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
			const format = "tag: %s mode: %s info: %s\n"
			t.Logf(format, tag, client.Mode, client.Info())
		}
	})

	clients := pool.Clients()
	require.Len(t, clients, testClientNum)

	testsuite.IsDestroyed(t, pool)
}

func TestPool_Delete(t *testing.T) {
	pool := testGeneratePool(t)

	t.Run("basic", func(t *testing.T) {
		for _, tag := range testClientTags {
			err := pool.Delete(tag)
			require.NoError(t, err)
		}
		clients := pool.Clients()
		require.Len(t, clients, testReserveClientNum)
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

	clients := pool.Clients()
	require.Len(t, clients, testReserveClientNum)

	testsuite.IsDestroyed(t, pool)
}

func TestPool_Add_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test1"
		tag2 = "test2"
	)

	certPool := testcert.CertPool(t)
	client1 := &Client{
		Tag:     tag1,
		Mode:    ModeSocks5,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}
	client2 := &Client{
		Tag:     tag2,
		Mode:    ModeHTTP,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}

	t.Run("part", func(t *testing.T) {
		pool := NewPool(certPool)

		add1 := func() {
			err := pool.Add(client1)
			require.NoError(t, err)
		}
		add2 := func() {
			err := pool.Add(client2)
			require.NoError(t, err)
		}
		cleanup := func() {
			clients := pool.Clients()
			require.Len(t, clients, testReserveClientNum+2)

			err := pool.Delete(tag1)
			require.NoError(t, err)
			err = pool.Delete(tag2)
			require.NoError(t, err)

			clients = pool.Clients()
			require.Len(t, clients, testReserveClientNum)
		}
		testsuite.RunParallel(100, nil, cleanup, add1, add2)

		testsuite.IsDestroyed(t, pool)
	})

	t.Run("whole", func(t *testing.T) {
		var pool *Pool

		init := func() {
			pool = NewPool(certPool)
		}
		add1 := func() {
			err := pool.Add(client1)
			require.NoError(t, err)
		}
		add2 := func() {
			err := pool.Add(client2)
			require.NoError(t, err)
		}
		testsuite.RunParallel(100, init, nil, add1, add2)

		testsuite.IsDestroyed(t, pool)
	})

	testsuite.IsDestroyed(t, certPool)
	testsuite.IsDestroyed(t, client1)
	testsuite.IsDestroyed(t, client2)
}

func TestPool_Delete_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test1"
		tag2 = "test2"
	)

	certPool := testcert.CertPool(t)
	client1 := &Client{
		Tag:     tag1,
		Mode:    ModeSocks5,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}
	client2 := &Client{
		Tag:     tag2,
		Mode:    ModeHTTP,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}

	t.Run("part", func(t *testing.T) {
		pool := NewPool(certPool)

		init := func() {
			err := pool.Add(client1)
			require.NoError(t, err)
			err = pool.Add(client2)
			require.NoError(t, err)
		}
		delete1 := func() {
			err := pool.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := pool.Delete(tag2)
			require.NoError(t, err)
		}
		cleanup := func() {
			clients := pool.Clients()
			require.Len(t, clients, testReserveClientNum)
		}
		testsuite.RunParallel(100, init, cleanup, delete1, delete2)

		testsuite.IsDestroyed(t, pool)
	})

	t.Run("whole", func(t *testing.T) {
		var pool *Pool

		init := func() {
			pool = NewPool(certPool)

			err := pool.Add(client1)
			require.NoError(t, err)
			err = pool.Add(client2)
			require.NoError(t, err)
		}
		delete1 := func() {
			err := pool.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := pool.Delete(tag2)
			require.NoError(t, err)
		}
		cleanup := func() {
			clients := pool.Clients()
			require.Len(t, clients, testReserveClientNum)
		}
		testsuite.RunParallel(100, init, cleanup, delete1, delete2)

		testsuite.IsDestroyed(t, pool)
	})

	testsuite.IsDestroyed(t, certPool)
	testsuite.IsDestroyed(t, client1)
	testsuite.IsDestroyed(t, client2)
}

func TestPool_Get_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test1"
		tag2 = "test2"
	)

	certPool := testcert.CertPool(t)
	client1 := &Client{
		Tag:     tag1,
		Mode:    ModeSocks5,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}
	client2 := &Client{
		Tag:     tag2,
		Mode:    ModeHTTP,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}

	t.Run("part", func(t *testing.T) {
		pool := NewPool(certPool)

		err := pool.Add(client1)
		require.NoError(t, err)
		err = pool.Add(client2)
		require.NoError(t, err)

		get1 := func() {
			client, err := pool.Get(tag1)
			require.NoError(t, err)
			require.NotNil(t, client)
			require.Equal(t, tag1, client.Tag)
		}
		get2 := func() {
			client, err := pool.Get(tag2)
			require.NoError(t, err)
			require.NotNil(t, client)
			require.Equal(t, tag2, client.Tag)
		}
		cleanup := func() {
			clients := pool.Clients()
			require.Len(t, clients, testReserveClientNum+2)
		}
		testsuite.RunParallel(100, nil, cleanup, get1, get2)

		testsuite.IsDestroyed(t, pool)
	})

	t.Run("whole", func(t *testing.T) {
		var pool *Pool

		init := func() {
			pool = NewPool(certPool)

			err := pool.Add(client1)
			require.NoError(t, err)
			err = pool.Add(client2)
			require.NoError(t, err)
		}
		get1 := func() {
			client, err := pool.Get(tag1)
			require.NoError(t, err)
			require.NotNil(t, client)
			require.Equal(t, tag1, client.Tag)
		}
		get2 := func() {
			client, err := pool.Get(tag2)
			require.NoError(t, err)
			require.NotNil(t, client)
			require.Equal(t, tag2, client.Tag)
		}
		cleanup := func() {
			clients := pool.Clients()
			require.Len(t, clients, testReserveClientNum+2)
		}
		testsuite.RunParallel(100, init, cleanup, get1, get2)

		testsuite.IsDestroyed(t, pool)
	})

	testsuite.IsDestroyed(t, certPool)
	testsuite.IsDestroyed(t, client1)
	testsuite.IsDestroyed(t, client2)
}

func TestPool_Clients_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test1"
		tag2 = "test2"
	)

	certPool := testcert.CertPool(t)
	client1 := &Client{
		Tag:     tag1,
		Mode:    ModeSocks5,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}
	client2 := &Client{
		Tag:     tag2,
		Mode:    ModeHTTP,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}

	t.Run("part", func(t *testing.T) {
		pool := NewPool(certPool)

		err := pool.Add(client1)
		require.NoError(t, err)
		err = pool.Add(client2)
		require.NoError(t, err)

		clients := func() {
			clients := pool.Clients()
			require.Len(t, clients, testReserveClientNum+2)
		}
		testsuite.RunParallel(100, nil, nil, clients, clients)

		testsuite.IsDestroyed(t, pool)
	})

	t.Run("whole", func(t *testing.T) {
		var pool *Pool

		init := func() {
			pool = NewPool(certPool)

			err := pool.Add(client1)
			require.NoError(t, err)
			err = pool.Add(client2)
			require.NoError(t, err)
		}
		clients := func() {
			clients := pool.Clients()
			require.Len(t, clients, testReserveClientNum+2)
		}
		testsuite.RunParallel(100, init, nil, clients, clients)

		testsuite.IsDestroyed(t, pool)
	})

	testsuite.IsDestroyed(t, certPool)
	testsuite.IsDestroyed(t, client1)
	testsuite.IsDestroyed(t, client2)
}

func TestPool_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		tag1 = "test1" // for init and delete
		tag2 = "test2" // for init and delete
		tag3 = "test3" // for add
		tag4 = "test4" // for add
		tag5 = "test5" // for get
		tag6 = "test6" // for get
	)

	certPool := testcert.CertPool(t)
	client1 := &Client{
		Tag:     tag1,
		Mode:    ModeSocks5,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}
	client2 := &Client{
		Tag:     tag2,
		Mode:    ModeHTTP,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}
	client3 := &Client{
		Tag:     tag3,
		Mode:    ModeSocks4a,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}
	client4 := &Client{
		Tag:     tag4,
		Mode:    ModeHTTPS,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}
	client5 := &Client{
		Tag:     tag5,
		Mode:    ModeSocks4,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}
	client6 := &Client{
		Tag:     tag6,
		Mode:    ModeHTTPS,
		Network: "tcp",
		Address: "127.0.0.1:1080",
	}

	t.Run("part", func(t *testing.T) {
		pool := NewPool(certPool)

		init := func() {
			err := pool.Add(client1)
			require.NoError(t, err)
			err = pool.Add(client2)
			require.NoError(t, err)
			err = pool.Add(client5)
			require.NoError(t, err)
			err = pool.Add(client6)
			require.NoError(t, err)
		}
		add1 := func() {
			err := pool.Add(client3)
			require.NoError(t, err)
		}
		add2 := func() {
			err := pool.Add(client4)
			require.NoError(t, err)
		}
		delete1 := func() {
			err := pool.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := pool.Delete(tag2)
			require.NoError(t, err)
		}
		get1 := func() {
			client, err := pool.Get(tag5)
			require.NoError(t, err)
			require.NotNil(t, client)
			require.Equal(t, tag5, client.Tag)
		}
		get2 := func() {
			client, err := pool.Get(tag6)
			require.NoError(t, err)
			require.NotNil(t, client)
			require.Equal(t, tag6, client.Tag)
		}
		clients := func() {
			clients := pool.Clients()
			require.NotEmpty(t, clients)
		}
		cleanup := func() {
			err := pool.Delete(tag3)
			require.NoError(t, err)
			err = pool.Delete(tag4)
			require.NoError(t, err)
			err = pool.Delete(tag5)
			require.NoError(t, err)
			err = pool.Delete(tag6)
			require.NoError(t, err)

			clients := pool.Clients()
			require.Len(t, clients, testReserveClientNum)
		}
		fns := []func(){
			add1, add2, delete1, delete2,
			get1, get2, clients, clients,
		}
		testsuite.RunParallel(100, init, cleanup, fns...)

		testsuite.IsDestroyed(t, pool)
	})

	t.Run("whole", func(t *testing.T) {
		var pool *Pool

		init := func() {
			pool = NewPool(certPool)

			err := pool.Add(client1)
			require.NoError(t, err)
			err = pool.Add(client2)
			require.NoError(t, err)
			err = pool.Add(client5)
			require.NoError(t, err)
			err = pool.Add(client6)
			require.NoError(t, err)
		}
		add1 := func() {
			err := pool.Add(client3)
			require.NoError(t, err)
		}
		add2 := func() {
			err := pool.Add(client4)
			require.NoError(t, err)
		}
		delete1 := func() {
			err := pool.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := pool.Delete(tag2)
			require.NoError(t, err)
		}
		get1 := func() {
			client, err := pool.Get(tag5)
			require.NoError(t, err)
			require.NotNil(t, client)
			require.Equal(t, tag5, client.Tag)
		}
		get2 := func() {
			client, err := pool.Get(tag6)
			require.NoError(t, err)
			require.NotNil(t, client)
			require.Equal(t, tag6, client.Tag)
		}
		clients := func() {
			clients := pool.Clients()
			require.NotEmpty(t, clients)
		}
		cleanup := func() {
			err := pool.Delete(tag3)
			require.NoError(t, err)
			err = pool.Delete(tag4)
			require.NoError(t, err)
			err = pool.Delete(tag5)
			require.NoError(t, err)
			err = pool.Delete(tag6)
			require.NoError(t, err)

			clients := pool.Clients()
			require.Len(t, clients, testReserveClientNum)
		}
		fns := []func(){
			add1, add2, delete1, delete2,
			get1, get2, clients, clients,
		}
		testsuite.RunParallel(100, init, cleanup, fns...)

		testsuite.IsDestroyed(t, pool)
	})

	testsuite.IsDestroyed(t, certPool)
	testsuite.IsDestroyed(t, client1)
	testsuite.IsDestroyed(t, client2)
	testsuite.IsDestroyed(t, client3)
	testsuite.IsDestroyed(t, client4)
	testsuite.IsDestroyed(t, client5)
	testsuite.IsDestroyed(t, client6)
}
