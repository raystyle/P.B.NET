package socks

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestV4Reply_String(t *testing.T) {
	t.Run("v4Refused", func(t *testing.T) {
		reply := v4Reply(v4Refused)
		t.Log(reply)
	})

	t.Run("v4Ident", func(t *testing.T) {
		reply := v4Reply(v4Ident)
		t.Log(reply)
	})

	t.Run("v4InvalidID", func(t *testing.T) {
		reply := v4Reply(v4InvalidID)
		t.Log(reply)
	})

	t.Run("unknown", func(t *testing.T) {
		reply := v4Reply(0xff)
		t.Log(reply)
	})
}

func TestClient_connectSocks4(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("IPv6", func(t *testing.T) {
		client := Client{}

		err := client.connectSocks4(nil, "::1", 1)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("don't support hostname", func(t *testing.T) {
		client := Client{disableExt: true}

		err := client.connectSocks4(nil, "host", 1)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("hostname too long", func(t *testing.T) {
		client := Client{}
		hostname := strings.Repeat("a", 257)

		err := client.connectSocks4(nil, hostname, 1)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("write to request", func(t *testing.T) {
		client := Client{}
		conn := testsuite.NewMockConnWithWriteError()

		err := client.connectSocks4(conn, "host", 1)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("invalid reply", func(t *testing.T) {
		client := Client{}
		srv, cli := net.Pipe()
		defer func() {
			err := srv.Close()
			require.NoError(t, err)
			err = cli.Close()
			require.NoError(t, err)
		}()

		go func() {
			_, err := io.CopyN(ioutil.Discard, srv, 9)
			require.NoError(t, err)

			reply := make([]byte, 1+1+2+net.IPv4len)
			reply[0] = 0x01

			_, err = srv.Write(reply)
			require.NoError(t, err)
		}()

		err := client.connectSocks4(cli, "1.1.1.1", 1)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("ok", func(t *testing.T) {
		client := Client{}
		srv, cli := net.Pipe()
		defer func() {
			err := srv.Close()
			require.NoError(t, err)
			err = cli.Close()
			require.NoError(t, err)
		}()

		go func() {
			_, err := io.CopyN(ioutil.Discard, srv, 9)
			require.NoError(t, err)

			_, err = srv.Write(v4ReplySucceeded)
			require.NoError(t, err)
		}()

		err := client.connectSocks4(cli, "1.1.1.1", 1)
		require.NoError(t, err)

		testsuite.IsDestroyed(t, &client)
	})
}

func testSocks4ClientWrite(t *testing.T, server *Server, write func(cli net.Conn)) {
	srv, cli := net.Pipe()
	defer func() {
		err := srv.Close()
		require.NoError(t, err)
		err = cli.Close()
		require.NoError(t, err)
	}()

	go write(cli)

	conn := &conn{
		server: server,
		local:  srv,
	}

	conn.serveSocks4()
}

func TestConn_serveSocks4(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewSocks4aServer("test", logger.Test, nil)
	require.NoError(t, err)

	t.Run("failed to read request", func(t *testing.T) {
		conn := &conn{
			server: server,
			local:  testsuite.NewMockConnWithReadError(),
		}

		conn.serveSocks4()
	})

	t.Run("invalid version", func(t *testing.T) {
		testSocks4ClientWrite(t, server, func(cli net.Conn) {
			req := make([]byte, 8)
			req[0] = 0x00

			_, err := cli.Write(req)
			require.NoError(t, err)
		})
	})

	t.Run("invalid command", func(t *testing.T) {
		testSocks4ClientWrite(t, server, func(cli net.Conn) {
			req := make([]byte, 8)
			req[0] = version4
			req[1] = 0x00

			_, err := cli.Write(req)
			require.NoError(t, err)
		})
	})

	t.Run("failed to read domain", func(t *testing.T) {
		testSocks4ClientWrite(t, server, func(cli net.Conn) {
			req := make([]byte, 8+1) // user id
			req[0] = version4
			req[1] = connect
			// port
			req[2] = 0x01
			req[3] = 0x01
			// ip address
			req[7] = 0x01

			_, err := cli.Write(req)
			require.NoError(t, err)

			err = cli.Close()
			require.NoError(t, err)
		})
	})

	t.Run("failed to write reply", func(t *testing.T) {
		opts := Options{DialContext: func(context.Context, string, string) (net.Conn, error) {
			return testsuite.NewMockConn(), nil
		}}

		server, err := NewSocks4aServer("test", logger.Test, &opts)
		require.NoError(t, err)

		testSocks4ClientWrite(t, server, func(cli net.Conn) {
			req := make([]byte, 8+1) // user id
			req[0] = version4
			req[1] = connect
			// port
			req[2] = 0x01
			req[3] = 0x01
			// ip address
			req[4] = 0x01
			req[5] = 0x01
			req[6] = 0x01
			req[7] = 0x01

			_, err = cli.Write(req)
			require.NoError(t, err)

			err = cli.Close()
			require.NoError(t, err)
		})

		testsuite.IsDestroyed(t, server)
	})

	testsuite.IsDestroyed(t, server)
}

func TestConn_checkUserID(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewSocks4aServer("test", logger.Test, nil)
	require.NoError(t, err)

	testSocks4ClientWrite(t, server, func(cli net.Conn) {
		req := make([]byte, 8)
		req[0] = version4
		req[1] = connect
		// port
		req[2] = 0x01
		req[3] = 0x01
		// ip address
		req[7] = 0x01

		_, err = cli.Write(req)
		require.NoError(t, err)

		err = cli.Close()
		require.NoError(t, err)
	})

	testsuite.IsDestroyed(t, server)
}
