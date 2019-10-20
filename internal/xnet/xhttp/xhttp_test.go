package xhttp

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/xnet/testdata"
)

func TestXHTTP(t *testing.T) {
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
	url := "http://" + listener.Addr().String()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	conn, err := Dial(req, &http.Transport{}, 0)
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
