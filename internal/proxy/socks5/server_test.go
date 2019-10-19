package socks5

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testutil"
)

func TestServer(t *testing.T) {
	server := testGenerateServer(t)
	require.NoError(t, server.ListenAndServe("localhost:0"))
	t.Log("address:", server.Address())
	t.Log("info:", server.Info())
	require.NoError(t, server.Close())
	require.NoError(t, server.Close())
	testutil.IsDestroyed(t, server, 2)
}

func testGenerateServer(t *testing.T) *Server {
	opts := Options{
		Username: "admin",
		Password: "123456",
	}
	server, err := NewServer("test", logger.Test, &opts)
	require.NoError(t, err)
	return server
}
