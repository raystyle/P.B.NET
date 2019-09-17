package light

import (
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/xnet/testdata"
)

func TestLight(t *testing.T) {
	listener, err := Listen("tcp", "localhost:0", 0)
	require.NoError(t, err)
	go func() {
		conn, err := listener.Accept()
		require.NoError(t, err)
		write := func() {
			data := testdata.GenerateData()
			_, err = conn.Write(data)
			require.NoError(t, err)
			require.Equal(t, testdata.GenerateData(), data)
		}
		read := func() {
			data := make([]byte, 256)
			_, err = io.ReadFull(conn, data)
			require.NoError(t, err)
			require.Equal(t, testdata.GenerateData(), data)
		}
		read()
		write()
		write()
		read()
	}()
	conn, err := Dial("tcp", listener.Addr().String(), 0)
	require.NoError(t, err)
	write := func() {
		data := testdata.GenerateData()
		_, err = conn.Write(data)
		require.NoError(t, err)
		// check data is changed after write
		require.Equal(t, testdata.GenerateData(), data)
	}
	read := func() {
		data := make([]byte, 256)
		_, err = io.ReadFull(conn, data)
		require.NoError(t, err)
		require.Equal(t, testdata.GenerateData(), data)
	}
	write()
	read()
	read()
	write()
}

func TestLightConn(t *testing.T) {
	server, client := net.Pipe()
	go func() {
		conn := Server(server, 0)
		write := func() {
			data := testdata.GenerateData()
			_, err := conn.Write(data)
			require.NoError(t, err)
			require.Equal(t, testdata.GenerateData(), data)
		}
		read := func() {
			data := make([]byte, 256)
			_, err := io.ReadFull(conn, data)
			require.NoError(t, err)
			require.Equal(t, testdata.GenerateData(), data)
		}
		read()
		write()
		write()
		read()
	}()
	conn := Client(client, 0)
	write := func() {
		data := testdata.GenerateData()
		_, err := conn.Write(data)
		require.NoError(t, err)
		// check data is changed after write
		require.Equal(t, testdata.GenerateData(), data)
	}
	read := func() {
		data := make([]byte, 256)
		_, err := io.ReadFull(conn, data)
		require.NoError(t, err)
		require.Equal(t, testdata.GenerateData(), data)
	}
	write()
	read()
	read()
	write()
}
