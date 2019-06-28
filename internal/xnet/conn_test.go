package xnet

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/protocol"
)

var (
	test_data = []byte("test data")
)

func Test_Conn(t *testing.T) {
	config := &Config{
		Network: "tcp",
		Address: ":0",
	}
	// Listen
	listener, err := Listen(LIGHT, config)
	require.Nil(t, err, err)
	go func() {
		conn, err := listener.Accept()
		require.Nil(t, err, err)
		c := New_Conn(conn, time.Now().Unix(), protocol.V1_0_0)
		err = c.Send(test_data)
		require.Nil(t, err, err)
	}()
	// Dial
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.Nil(t, err, err)
	config.Address = "localhost:" + port
	conn, err := Dial(LIGHT, config)
	require.Nil(t, err, err)
	c := New_Conn(conn, time.Now().Unix(), protocol.V1_0_0)
	msg, err := c.Receive()
	require.Nil(t, err, err)
	require.Equal(t, test_data, msg)
	t.Log(c.Info())
	_ = conn.Close()
}
