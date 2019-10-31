package proxy

import (
	"io/ioutil"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestManager(t *testing.T) {
	const (
		tagSocks = "test_socks"
		tagHTTP  = "test_http"
	)
	options, err := ioutil.ReadFile("testdata/socks5_opts.toml")
	require.NoError(t, err)
	socksServer := &Server{
		Tag:     tagSocks,
		Mode:    ModeSocks,
		Options: string(options),
	}
	options, err = ioutil.ReadFile("testdata/http_opts.toml")
	require.NoError(t, err)
	httpServer := &Server{
		Tag:     tagHTTP,
		Mode:    ModeHTTP,
		Options: string(options),
	}
	manager := NewManager(logger.Test, nil)
	err = manager.Add(socksServer)
	require.NoError(t, err)
	err = manager.Add(httpServer)
	require.NoError(t, err)
	// add client with empty tag
	testServer := &Server{}
	err = manager.Add(testServer)
	require.Errorf(t, err, "empty proxy server tag")
	// add unknown mode
	testServer.Tag = "foo"
	testServer.Mode = "foo mode"
	err = manager.Add(testServer)
	require.Errorf(t, err, "unknown mode: foo mode")
	// add exist
	err = manager.Add(socksServer)
	require.Errorf(t, err, "proxy server %s already exists", tagSocks)
	// get
	ps, err := manager.Get(tagSocks)
	require.NoError(t, err)
	require.NotNil(t, ps)
	err = ps.ListenAndServe("tcp", "localhost:0")
	require.NoError(t, err)
	t.Logf("create at: %s, serve at: %s", ps.CreateAt(), ps.ServeAt())

	ps, err = manager.Get(tagHTTP)
	require.NoError(t, err)
	require.NotNil(t, ps)
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	ps.Serve(listener)
	t.Logf("create at: %s, serve at: %s", ps.CreateAt(), ps.ServeAt())

	// get ""
	ps, err = manager.Get("")
	require.Error(t, err)
	require.Nil(t, ps)
	// get doesn't exist
	ps, err = manager.Get("foo")
	require.Errorf(t, err, "proxy server foo doesn't exist")
	require.Nil(t, ps)
	// get all servers info
	for tag, server := range manager.Servers() {
		t.Logf("tag: %s mode: %s info: %s", tag, server.Mode, server.Info())
	}
	// delete
	err = manager.Delete(tagHTTP)
	require.NoError(t, err)
	// delete doesn't exist
	err = manager.Delete(tagHTTP)
	require.Errorf(t, err, "proxy server %s doesn't exist", tagHTTP)
	// delete client with empty tag
	err = manager.Delete("")
	require.Errorf(t, err, "empty proxy server tag")

	// check object
	require.NoError(t, manager.Close())
	testsuite.IsDestroyed(t, manager)
}

func TestManager_Add(t *testing.T) {
	manager := NewManager(logger.Test, nil)
	// add socks server with invalid toml data
	err := manager.Add(&Server{
		Tag:     "invalid socks5",
		Mode:    ModeSocks,
		Options: "socks4 = foo",
	})
	require.Error(t, err)

	// add http proxy server with invalid toml data
	err = manager.Add(&Server{
		Tag:     "invalid http",
		Mode:    ModeHTTP,
		Options: "https = foo",
	})
	require.Error(t, err)

	// add http proxy server with invalid options
	err = manager.Add(&Server{
		Tag:  "http with invalid options",
		Mode: ModeHTTP,
		Options: `
[transport]
  [transport.tls_config]
    root_ca = ["foo data"]
`,
	})
	require.Error(t, err)
}
