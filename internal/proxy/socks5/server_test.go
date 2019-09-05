package socks5

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
)

func TestServer(t *testing.T) {
	s := testGenerateServer(t)
	err := s.ListenAndServe("localhost:0", 0)
	require.NoError(t, err)
	t.Log(s.Info())
	t.Log(s.Addr())
	// select {}
	err = s.Stop()
	require.NoError(t, err)
}

func testGenerateServer(t *testing.T) *Server {
	opts := &Options{
		Username: "admin",
		Password: "123456",
	}
	s, err := NewServer("test", logger.Test, opts)
	require.NoError(t, err)
	return s
}
