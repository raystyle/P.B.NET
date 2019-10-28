package proxy

import (
	"io/ioutil"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testutil"
)

func TestManager(t *testing.T) {
	const (
		tagSocks = "test_socks"
		tagHTTP  = "test_http"
	)
	options, err := ioutil.ReadFile("testdata/socks5_opts.toml")
	require.NoError(t, err)
	socksServer := &Server{
		Mode:    ModeSocks,
		Options: string(options),
	}
	options, err = ioutil.ReadFile("testdata/http_opts.toml")
	require.NoError(t, err)
	httpServer := &Server{
		Mode:    ModeHTTP,
		Options: string(options),
	}
	manager := NewManager(logger.Test)
	err = manager.Add(tagSocks, socksServer)
	require.NoError(t, err)
	err = manager.Add(tagHTTP, httpServer)
	require.NoError(t, err)
	// add client with empty tag
	err = manager.Add("", socksServer)
	require.Errorf(t, err, "empty proxy server tag")
	// add unknown mode
	err = manager.Add("foo", &Server{Mode: "foo mode"})
	require.Errorf(t, err, "unknown mode: foo mode")
	// add exist
	err = manager.Add(tagSocks, socksServer)
	require.Errorf(t, err, "proxy server %s already exists", tagSocks)
	// get
	ps, err := manager.Get(tagSocks)
	require.NoError(t, err)
	require.NotNil(t, ps)
	err = ps.ListenAndServe("tcp", "localhost:0")
	require.NoError(t, err)
	t.Logf("create at: %s, serve at: %s", ps.CreateAt, ps.ServeAt())

	ps, err = manager.Get(tagHTTP)
	require.NoError(t, err)
	require.NotNil(t, ps)
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	ps.Serve(listener)
	t.Logf("create at: %s, serve at: %s", ps.CreateAt, ps.ServeAt())

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
	err = manager.Delete(tagSocks)
	require.NoError(t, err)
	testutil.IsDestroyed(t, manager)
}
