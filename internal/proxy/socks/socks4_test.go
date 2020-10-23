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

func testSocks4ServerWrite(t *testing.T, client *Client, write func(net.Conn)) {
	testsuite.PipeWithReaderWriter(t,
		func(conn net.Conn) {
			err := client.connectSocks4(conn, "1.1.1.1", 1)
			require.Error(t, err)
		},
		func(conn net.Conn) {
			write(conn)
		},
	)
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
		client := new(Client)

		testSocks4ServerWrite(t, client, func(server net.Conn) {
			_, err := io.CopyN(ioutil.Discard, server, 9)
			require.NoError(t, err)

			reply := make([]byte, 1+1+2+net.IPv4len)
			reply[0] = 0x01

			_, err = server.Write(reply)
			require.NoError(t, err)
		})

		testsuite.IsDestroyed(t, client)
	})

	t.Run("ok", func(t *testing.T) {
		client := new(Client)

		testsuite.PipeWithReaderWriter(t,
			func(conn net.Conn) {
				err := client.connectSocks4(conn, "1.1.1.1", 1)
				require.NoError(t, err)
			},
			func(server net.Conn) {
				_, err := io.CopyN(ioutil.Discard, server, 9)
				require.NoError(t, err)

				_, err = server.Write(v4ReplySucceeded)
				require.NoError(t, err)
			},
		)

		testsuite.IsDestroyed(t, client)
	})
}

func testSocks4ClientWrite(t *testing.T, server *Server, write func(net.Conn)) {
	testsuite.PipeWithReaderWriter(t,
		func(c net.Conn) {
			conn := &conn{ctx: server, local: c}
			conn.serveSocks4()
		},
		func(conn net.Conn) {
			write(conn)
		},
	)
}

func TestConn_serveSocks4(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewSocks4aServer("test", logger.Test, nil)
	require.NoError(t, err)

	t.Run("failed to read request", func(t *testing.T) {
		conn := &conn{
			ctx:   server,
			local: testsuite.NewMockConnWithReadError(),
		}
		conn.serveSocks4()
	})

	t.Run("invalid version", func(t *testing.T) {
		testSocks4ClientWrite(t, server, func(client net.Conn) {
			req := make([]byte, 8)
			req[0] = 0x00

			_, err := client.Write(req)
			require.NoError(t, err)
		})
	})

	t.Run("invalid command", func(t *testing.T) {
		testSocks4ClientWrite(t, server, func(client net.Conn) {
			req := make([]byte, 8)
			req[0] = version4
			req[1] = 0x00

			_, err := client.Write(req)
			require.NoError(t, err)
		})
	})

	t.Run("failed to read domain", func(t *testing.T) {
		testSocks4ClientWrite(t, server, func(client net.Conn) {
			req := make([]byte, 8+1) // user id
			req[0] = version4
			req[1] = connect
			// port
			req[2] = 0x01
			req[3] = 0x01
			// ip address
			req[7] = 0x01

			_, err := client.Write(req)
			require.NoError(t, err)

			err = client.Close()
			require.NoError(t, err)
		})
	})

	t.Run("failed to write reply", func(t *testing.T) {
		opts := Options{DialContext: func(context.Context, string, string) (net.Conn, error) {
			return testsuite.NewMockConn(), nil
		}}

		server, err := NewSocks4aServer("test", logger.Test, &opts)
		require.NoError(t, err)

		testSocks4ClientWrite(t, server, func(client net.Conn) {
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

			_, err = client.Write(req)
			require.NoError(t, err)

			err = client.Close()
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

	testSocks4ClientWrite(t, server, func(client net.Conn) {
		req := make([]byte, 8)
		req[0] = version4
		req[1] = connect
		// port
		req[2] = 0x01
		req[3] = 0x01
		// ip address
		req[7] = 0x01

		_, err = client.Write(req)
		require.NoError(t, err)

		err = client.Close()
		require.NoError(t, err)
	})

	testsuite.IsDestroyed(t, server)
}
