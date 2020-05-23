package socks

import (
	"io"
	"io/ioutil"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestV5Reply_String(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		for i := 1; i < 9; i++ {
			reply := v5Reply(i)
			t.Log(reply)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		reply := v5Reply(0xff)
		t.Log(reply)
	})
}

func testSocks5ServerWrite(t *testing.T, client *Client, host string, write func(server net.Conn)) {
	srv, cli := net.Pipe()
	defer func() {
		err := srv.Close()
		require.NoError(t, err)
		err = cli.Close()
		require.NoError(t, err)
	}()

	go func() { _, _ = io.Copy(ioutil.Discard, srv) }()
	go write(srv)

	err := client.connectSocks5(cli, host, 1)
	require.Error(t, err)
}

func TestClient_connectSocks(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const host = "1.1.1.1"

	t.Run("failed to read reply", func(t *testing.T) {
		client := Client{}
		conn := testsuite.NewMockConnWithReadError()

		err := client.connectSocks5(conn, host, 1)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("invalid version", func(t *testing.T) {
		client := new(Client)

		testSocks5ServerWrite(t, client, host, func(server net.Conn) {
			reply := make([]byte, 2)
			reply[0] = 0x00

			_, err := server.Write(reply)
			require.NoError(t, err)
		})

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("host too long", func(t *testing.T) {
		client := new(Client)
		host := strings.Repeat("a", 257)

		testSocks5ServerWrite(t, client, host, func(server net.Conn) {
			reply := make([]byte, 2)
			reply[0] = version5
			reply[1] = notRequired

			_, err := server.Write(reply)
			require.NoError(t, err)
		})

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("failed to send connect target", func(t *testing.T) {
		client := Client{}
		srv, cli := net.Pipe()
		defer func() {
			err := srv.Close()
			require.NoError(t, err)
			err = cli.Close()
			require.NoError(t, err)
		}()

		go func() {
			_, err := io.CopyN(ioutil.Discard, srv, 3)
			require.NoError(t, err)

			reply := make([]byte, 2)
			reply[0] = version5
			reply[1] = notRequired

			_, err = srv.Write(reply)
			require.NoError(t, err)

			err = srv.Close()
			require.NoError(t, err)
		}()

		err := client.connectSocks5(cli, host, 1)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})
}
