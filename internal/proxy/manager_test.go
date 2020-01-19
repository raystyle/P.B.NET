package proxy

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

var testServerTags = []string{
	"socks5",
	"socks4a",
	"socks4",
	"http",
	"https",
}

func testGenerateManager(t *testing.T) *Manager {
	manager := NewManager(logger.Test, nil)
	for i, filename := range []string{
		"socks/testdata/socks5_client.toml",
		"socks/testdata/socks4a_client.toml",
		"socks/testdata/socks4_client.toml",
		"http/testdata/http_client.toml",
		"http/testdata/https_client.toml",
	} {
		opts, err := ioutil.ReadFile(filename)
		require.NoError(t, err)
		server := &Server{
			Tag:     testServerTags[i],
			Mode:    testServerTags[i],
			Options: string(opts),
		}
		err = manager.Add(server)
		require.NoError(t, err)
	}
	return manager
}

func TestManager_Add(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := testGenerateManager(t)

	t.Run("with empty tag", func(t *testing.T) {
		err := manager.add(new(Server))
		require.EqualError(t, err, "empty proxy server tag")
	})

	t.Run("with unknown mode", func(t *testing.T) {
		err := manager.add(&Server{
			Tag:  "foo",
			Mode: "foo mode",
		})
		require.EqualError(t, err, "unknown mode: foo mode")
	})

	t.Run("exist", func(t *testing.T) {
		opts, err := ioutil.ReadFile("socks/testdata/socks5_server.toml")
		require.NoError(t, err)
		server := &Server{
			Tag:     ModeSocks5,
			Mode:    ModeSocks5,
			Options: string(opts),
		}
		err = manager.Add(server)
		require.EqualError(t, err, "failed to add proxy server socks5: already exists")
	})

	t.Run("socks server with invalid toml data", func(t *testing.T) {
		err := manager.Add(&Server{
			Tag:     "invalid socks5",
			Mode:    ModeSocks5,
			Options: "socks4 = foo",
		})
		require.Error(t, err)
	})

	t.Run("http proxy server with invalid toml data", func(t *testing.T) {
		err := manager.Add(&Server{
			Tag:     "invalid http",
			Mode:    ModeHTTP,
			Options: "https = foo",
		})
		require.Error(t, err)
	})

	t.Run("http proxy server with invalid options", func(t *testing.T) {
		err := manager.Add(&Server{
			Tag:  "http with invalid options",
			Mode: ModeHTTP,
			Options: `
[transport]
  [transport.tls_config]
    root_ca = ["foo data"]
`,
		})
		require.Error(t, err)
	})

	require.NoError(t, manager.Close())
	testsuite.IsDestroyed(t, manager)
}

func TestManager_Get(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := testGenerateManager(t)

	t.Run("basic", func(t *testing.T) {
		for _, tag := range testServerTags {
			server, err := manager.Get(tag)
			require.NoError(t, err)
			require.NotNil(t, server)
		}
	})

	t.Run("empty tag", func(t *testing.T) {
		ps, err := manager.Get("")
		require.EqualError(t, err, "empty proxy server tag")
		require.Nil(t, ps)
	})

	t.Run("doesn't exist", func(t *testing.T) {
		ps, err := manager.Get("foo")
		require.EqualError(t, err, "proxy server foo doesn't exist")
		require.Nil(t, ps)
	})

	t.Run("print all", func(t *testing.T) {
		for tag, server := range manager.Servers() {
			t.Logf("tag: %s mode: %s info: %s\n", tag, server.Mode, server.Info())
		}
	})

	require.NoError(t, manager.Close())
	testsuite.IsDestroyed(t, manager)
}

func TestManager_Delete(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := testGenerateManager(t)

	t.Run("basic", func(t *testing.T) {
		for _, tag := range testServerTags {
			err := manager.Delete(tag)
			require.NoError(t, err)
		}
	})

	t.Run("empty tag", func(t *testing.T) {
		err := manager.Delete("")
		require.EqualError(t, err, "empty proxy server tag")
	})

	t.Run("doesn't exist", func(t *testing.T) {
		err := manager.Delete("foo")
		require.EqualError(t, err, "proxy server foo doesn't exist")
	})

	require.NoError(t, manager.Close())
	testsuite.IsDestroyed(t, manager)
}

func TestManager_Close(t *testing.T) {

}
