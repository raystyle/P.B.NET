package httpproxy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
)

func Test_Server(t *testing.T) {
	s := test_generate_server(t)
	err := s.Listen_And_Serve(":0", 0)
	require.Nil(t, err, err)
	t.Log(s.Info())
	t.Log(s.Addr())
	// select {}
	err = s.Stop()
	require.Nil(t, err, err)
}

func test_generate_server(t *testing.T) *Server {
	opts := &Options{
		Username: "admin",
		Password: "123456",
	}
	s, err := New_Server("test", logger.Test, opts)
	require.Nil(t, err, err)
	return s
}
