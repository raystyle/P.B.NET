package xnet

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	testdata = []byte("test data")
)

func TestConn(t *testing.T) {
	cfg := &Config{
		Network: "tcp",
		Address: "localhost:0",
	}
	// Listen
	listener, err := Listen(LIGHT, cfg)
	require.NoError(t, err)
	go func() {
		conn, err := listener.Accept()
		require.NoError(t, err)
		c := NewConn(conn, time.Now().Unix())
		err = c.Send(testdata)
		require.NoError(t, err)
	}()
	// Dial
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	cfg.Address = "localhost:" + port
	conn, err := Dial(LIGHT, cfg)
	require.NoError(t, err)
	c := NewConn(conn, time.Now().Unix())
	msg, err := c.Receive()
	require.NoError(t, err)
	require.Equal(t, testdata, msg)
	t.Log(c.Info())
	_ = conn.Close()
}
