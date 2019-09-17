package xnet

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/xnet/testdata"
)

func TestConn(t *testing.T) {
	server, client := net.Pipe()
	go func() {
		conn := NewConn(server, time.Now().Unix())
		write := func() {
			data := testdata.GenerateData()
			// check data is changed after write
			err := conn.Send(data)
			require.NoError(t, err)
			require.Equal(t, testdata.GenerateData(), data)
		}
		read := func() {
			data, err := conn.Receive()
			require.NoError(t, err)
			require.Equal(t, testdata.GenerateData(), data)
		}
		read()
		write()
		write()
		read()
	}()
	conn := NewConn(client, time.Now().Unix())
	write := func() {
		data := testdata.GenerateData()
		err := conn.Send(data)
		require.NoError(t, err)
		// check data is changed after write
		require.Equal(t, testdata.GenerateData(), data)
	}
	read := func() {
		data, err := conn.Receive()
		require.NoError(t, err)
		require.Equal(t, testdata.GenerateData(), data)
	}
	write()
	read()
	read()
	write()

	t.Log(conn.Info())
	_ = conn.Close()
}
