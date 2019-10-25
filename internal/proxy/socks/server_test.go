package socks5

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testutil"
)

func testGenerateServer(t *testing.T) *Server {
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	server, err := NewServer("test", logger.Test, &opts)
	require.NoError(t, err)
	require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	return server
}

func TestServer(t *testing.T) {
	server := testGenerateServer(t)
	t.Log("address:", server.Address())
	t.Log("info:", server.Info())
	require.NoError(t, server.Close())
	require.NoError(t, server.Close())
	testutil.IsDestroyed(t, server, 2)
}

func TestAuthenticate(t *testing.T) {
	server := testGenerateServer(t)
	defer func() {
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 1)
	}()
	opt := Options{
		Username: "admin",
		Password: "123457",
	}
	client, err := NewClient("tcp", server.Address(), &opt)
	require.NoError(t, err)
	_, err = client.Dial("tcp", "github.com:443")
	require.Error(t, err)
}
