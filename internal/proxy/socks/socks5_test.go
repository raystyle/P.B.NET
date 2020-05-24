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

func TestClient_connectSocks5(t *testing.T) {
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

	t.Run("IPv6", func(t *testing.T) {
		client := new(Client)

		testSocks5ServerWrite(t, client, "::1", func(server net.Conn) {
			reply := make([]byte, 2)
			reply[0] = version5
			reply[1] = notRequired

			_, err := server.Write(reply)
			require.NoError(t, err)

			err = server.Close()
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

func TestClient_authenticate(t *testing.T) {
	t.Run("empty username", func(t *testing.T) {
		client := Client{}

		err := client.authenticate(nil, usernamePassword)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("failed to write", func(t *testing.T) {
		client := Client{
			username: []byte("user"),
			password: []byte("pass"),
		}
		conn := testsuite.NewMockConnWithWriteError()

		err := client.authenticate(conn, usernamePassword)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("failed to read response", func(t *testing.T) {
		client := Client{
			username: []byte("user"),
			password: []byte("pass"),
		}
		conn := testsuite.NewMockConnWithReadError()

		err := client.authenticate(conn, usernamePassword)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("invalid response 0", func(t *testing.T) {
		client := Client{
			username: []byte("user"),
			password: []byte("pass"),
		}
		srv, cli := net.Pipe()
		defer func() {
			err := srv.Close()
			require.NoError(t, err)
			err = cli.Close()
			require.NoError(t, err)
		}()

		go func() {
			size := int64(1 + 1 + len(client.username) + 1 + len(client.password))
			_, err := io.CopyN(ioutil.Discard, srv, size)
			require.NoError(t, err)

			response := make([]byte, 2)

			_, err = srv.Write(response)
			require.NoError(t, err)
		}()

		err := client.authenticate(cli, usernamePassword)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("invalid response 1", func(t *testing.T) {
		client := Client{
			username: []byte("user"),
			password: []byte("pass"),
		}
		srv, cli := net.Pipe()
		defer func() {
			err := srv.Close()
			require.NoError(t, err)
			err = cli.Close()
			require.NoError(t, err)
		}()

		go func() {
			size := int64(1 + 1 + len(client.username) + 1 + len(client.password))
			_, err := io.CopyN(ioutil.Discard, srv, size)
			require.NoError(t, err)

			response := make([]byte, 2)
			response[0] = usernamePasswordVersion
			response[1] = 0x01

			_, err = srv.Write(response)
			require.NoError(t, err)
		}()

		err := client.authenticate(cli, usernamePassword)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("no acceptable methods", func(t *testing.T) {
		client := Client{}

		err := client.authenticate(nil, noAcceptableMethods)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("unsupported authentication method", func(t *testing.T) {
		client := Client{}

		err := client.authenticate(nil, 0x11)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})
}

func TestClient_receiveReply(t *testing.T) {
	t.Run("failed to receive reply", func(t *testing.T) {
		client := Client{}
		conn := testsuite.NewMockConnWithReadError()

		err := client.receiveReply(conn)
		require.Error(t, err)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("invalid version", func(t *testing.T) {
		client := Client{}

		testsuite.PipeWithReaderWriter(t,
			func(conn net.Conn) {
				err := client.receiveReply(conn)
				require.Error(t, err)
			},
			func(conn net.Conn) {
				reply := make([]byte, 4)

				_, err := conn.Write(reply)
				require.NoError(t, err)
			},
		)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("not succeeded", func(t *testing.T) {
		client := Client{}

		testsuite.PipeWithReaderWriter(t,
			func(conn net.Conn) {
				err := client.receiveReply(conn)
				require.Error(t, err)
			},
			func(conn net.Conn) {
				reply := make([]byte, 4)
				reply[0] = version5
				reply[1] = connRefused

				_, err := conn.Write(reply)
				require.NoError(t, err)
			},
		)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("invalid reserved", func(t *testing.T) {
		client := Client{}

		testsuite.PipeWithReaderWriter(t,
			func(conn net.Conn) {
				err := client.receiveReply(conn)
				require.Error(t, err)
			},
			func(conn net.Conn) {
				reply := make([]byte, 4)
				reply[0] = version5
				reply[1] = succeeded
				reply[2] = 0x01

				_, err := conn.Write(reply)
				require.NoError(t, err)
			},
		)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("invalid reply about padding", func(t *testing.T) {
		client := Client{}

		testsuite.PipeWithReaderWriter(t,
			func(conn net.Conn) {
				err := client.receiveReply(conn)
				require.Error(t, err)
			},
			func(conn net.Conn) {
				reply := make([]byte, 4)
				reply[0] = version5
				reply[1] = succeeded
				reply[2] = reserve

				_, err := conn.Write(reply)
				require.NoError(t, err)
			},
		)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("IPv6", func(t *testing.T) {
		client := Client{}

		testsuite.PipeWithReaderWriter(t,
			func(conn net.Conn) {
				err := client.receiveReply(conn)
				require.Error(t, err)
			},
			func(conn net.Conn) {
				reply := make([]byte, 4)
				reply[0] = version5
				reply[1] = succeeded
				reply[2] = reserve
				reply[3] = ipv6

				_, err := conn.Write(reply)
				require.NoError(t, err)

				err = conn.Close()
				require.NoError(t, err)
			},
		)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("FQDN", func(t *testing.T) {
		client := Client{}

		testsuite.PipeWithReaderWriter(t,
			func(conn net.Conn) {
				err := client.receiveReply(conn)
				require.Error(t, err)
			},
			func(conn net.Conn) {
				reply := make([]byte, 8)
				reply[0] = version5
				reply[1] = succeeded
				reply[2] = reserve
				reply[3] = fqdn
				reply[4] = 2
				reply[5] = 'a'
				reply[6] = '.'
				reply[7] = 'c'

				_, err := conn.Write(reply)
				require.NoError(t, err)

				err = conn.Close()
				require.NoError(t, err)
			},
		)

		testsuite.IsDestroyed(t, &client)
	})

	t.Run("FQDN read failed", func(t *testing.T) {
		client := Client{}

		testsuite.PipeWithReaderWriter(t,
			func(conn net.Conn) {
				err := client.receiveReply(conn)
				require.Error(t, err)
			},
			func(conn net.Conn) {
				reply := make([]byte, 4)
				reply[0] = version5
				reply[1] = succeeded
				reply[2] = reserve
				reply[3] = fqdn

				_, err := conn.Write(reply)
				require.NoError(t, err)

				err = conn.Close()
				require.NoError(t, err)
			},
		)

		testsuite.IsDestroyed(t, &client)
	})
}

func testSocks5ClientWrite(t *testing.T, server *Server, write func(cli net.Conn)) {
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

	conn.serveSocks5()
}

func TestServer_serveSocks5(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewSocks5Server("test", logger.Test, nil)
	require.NoError(t, err)

	t.Run("failed to read request", func(t *testing.T) {
		conn := &conn{
			server: server,
			local:  testsuite.NewMockConnWithReadError(),
		}

		conn.serveSocks5()
	})

	t.Run("invalid version", func(t *testing.T) {
		testSocks5ClientWrite(t, server, func(cli net.Conn) {
			req := make([]byte, 1)
			req[0] = 0x00

			_, err := cli.Write(req)
			require.NoError(t, err)
		})
	})

	t.Run("failed to read auth methods number", func(t *testing.T) {
		testSocks5ClientWrite(t, server, func(cli net.Conn) {
			req := make([]byte, 1)
			req[0] = version5

			_, err := cli.Write(req)
			require.NoError(t, err)

			err = cli.Close()
			require.NoError(t, err)
		})
	})

	t.Run("no authentication method", func(t *testing.T) {
		testSocks5ClientWrite(t, server, func(cli net.Conn) {
			req := make([]byte, 2)
			req[0] = version5

			_, err := cli.Write(req)
			require.NoError(t, err)
		})
	})

	t.Run("failed to read auth methods", func(t *testing.T) {
		testSocks5ClientWrite(t, server, func(cli net.Conn) {
			req := make([]byte, 2)
			req[0] = version5
			req[1] = 0xff

			_, err := cli.Write(req)
			require.NoError(t, err)

			err = cli.Close()
			require.NoError(t, err)
		})
	})

	t.Run("failed to receive target", func(t *testing.T) {
		testSocks5ClientWrite(t, server, func(cli net.Conn) {
			req := make([]byte, 3)
			req[0] = version5
			req[1] = 1
			req[2] = notRequired

			_, err := cli.Write(req)
			require.NoError(t, err)

			// receive auth
			_, err = io.CopyN(ioutil.Discard, cli, 2)
			require.NoError(t, err)

			err = cli.Close()
			require.NoError(t, err)
		})
	})

	t.Run("failed to write reply", func(t *testing.T) {
		opts := Options{DialContext: func(context.Context, string, string) (net.Conn, error) {
			return testsuite.NewMockConn(), nil
		}}

		server, err := NewSocks5Server("test", logger.Test, &opts)
		require.NoError(t, err)

		testSocks5ClientWrite(t, server, func(cli net.Conn) {
			req := make([]byte, 3)
			req[0] = version5
			req[1] = 1
			req[2] = notRequired

			_, err := cli.Write(req)
			require.NoError(t, err)

			// receive auth
			_, err = io.CopyN(ioutil.Discard, cli, 2)
			require.NoError(t, err)

			// write target
			req = make([]byte, 4+net.IPv4len+2)
			req[0] = version5
			req[1] = connect
			req[2] = reserve
			req[3] = ipv4
			// ip address
			req[4] = 1
			req[5] = 1
			req[6] = 1
			req[7] = 1
			// port
			req[8] = 0
			req[9] = 1

			_, err = cli.Write(req)
			require.NoError(t, err)

			err = cli.Close()
			require.NoError(t, err)
		})
	})

	testsuite.IsDestroyed(t, server)
}

func TestConn_authenticate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	opts := Options{Username: "u", Password: "p"}
	server, err := NewSocks5Server("test", logger.Test, &opts)
	require.NoError(t, err)

	t.Run("failed to write auth methods", func(t *testing.T) {
		conn := &conn{
			server: server,
			local:  testsuite.NewMockConnWithWriteError(),
		}

		ok := conn.authenticate()
		require.False(t, ok)
	})

	t.Run("failed to read user pass", func(t *testing.T) {
		conn := &conn{
			server: server,
			local:  testsuite.NewMockConnWithReadError(),
		}

		ok := conn.authenticate()
		require.False(t, ok)
	})

	t.Run("unexpected user pass version", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(c net.Conn) {
				conn := &conn{
					server: server,
					local:  c,
				}
				ok := conn.authenticate()
				require.False(t, ok)
			},
			func(conn net.Conn) {
				// receive auth
				_, err := io.CopyN(ioutil.Discard, conn, 2)
				require.NoError(t, err)

				req := make([]byte, 1)

				_, err = conn.Write(req)
				require.NoError(t, err)
			},
		)
	})

	t.Run("failed to read username length", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(c net.Conn) {
				conn := &conn{
					server: server,
					local:  c,
				}
				ok := conn.authenticate()
				require.False(t, ok)
			},
			func(conn net.Conn) {
				// receive auth
				_, err := io.CopyN(ioutil.Discard, conn, 2)
				require.NoError(t, err)

				req := make([]byte, 1)
				req[0] = usernamePasswordVersion

				_, err = conn.Write(req)
				require.NoError(t, err)

				err = conn.Close()
				require.NoError(t, err)
			},
		)
	})

	t.Run("failed to read username", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(c net.Conn) {
				conn := &conn{
					server: server,
					local:  c,
				}
				ok := conn.authenticate()
				require.False(t, ok)
			},
			func(conn net.Conn) {
				// receive auth
				_, err := io.CopyN(ioutil.Discard, conn, 2)
				require.NoError(t, err)

				req := make([]byte, 2)
				req[0] = usernamePasswordVersion
				req[1] = 255

				_, err = conn.Write(req)
				require.NoError(t, err)

				err = conn.Close()
				require.NoError(t, err)
			},
		)
	})

	t.Run("failed to read password length", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(c net.Conn) {
				conn := &conn{
					server: server,
					local:  c,
				}
				ok := conn.authenticate()
				require.False(t, ok)
			},
			func(conn net.Conn) {
				// receive auth
				_, err := io.CopyN(ioutil.Discard, conn, 2)
				require.NoError(t, err)

				req := make([]byte, 3)
				req[0] = usernamePasswordVersion
				req[1] = 1
				req[2] = 'u'

				_, err = conn.Write(req)
				require.NoError(t, err)

				err = conn.Close()
				require.NoError(t, err)
			},
		)
	})

	t.Run("failed to read password length", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(c net.Conn) {
				conn := &conn{
					server: server,
					local:  c,
				}
				ok := conn.authenticate()
				require.False(t, ok)
			},
			func(conn net.Conn) {
				// receive auth
				_, err := io.CopyN(ioutil.Discard, conn, 2)
				require.NoError(t, err)

				req := make([]byte, 4)
				req[0] = usernamePasswordVersion
				req[1] = 1
				req[2] = 'u'
				req[3] = 255

				_, err = conn.Write(req)
				require.NoError(t, err)

				err = conn.Close()
				require.NoError(t, err)
			},
		)
	})

	t.Run("failed to write user pass version", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(c net.Conn) {
				conn := &conn{
					server: server,
					local:  c,
				}
				ok := conn.authenticate()
				require.False(t, ok)
			},
			func(conn net.Conn) {
				// receive auth
				_, err := io.CopyN(ioutil.Discard, conn, 2)
				require.NoError(t, err)

				req := make([]byte, 5)
				req[0] = usernamePasswordVersion
				req[1] = 1
				req[2] = 'u'
				req[3] = 1
				req[4] = 'p'

				_, err = conn.Write(req)
				require.NoError(t, err)

				err = conn.Close()
				require.NoError(t, err)
			},
		)
	})

	t.Run("failed to write auth reply", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(c net.Conn) {
				conn := &conn{
					server: server,
					local:  c,
				}
				ok := conn.authenticate()
				require.False(t, ok)
			},
			func(conn net.Conn) {
				// receive auth
				_, err := io.CopyN(ioutil.Discard, conn, 2)
				require.NoError(t, err)

				req := make([]byte, 5)
				req[0] = usernamePasswordVersion
				req[1] = 1
				req[2] = 'u'
				req[3] = 1
				req[4] = 'p'

				_, err = conn.Write(req)
				require.NoError(t, err)

				// receive auth
				_, err = io.CopyN(ioutil.Discard, conn, 1)
				require.NoError(t, err)

				err = conn.Close()
				require.NoError(t, err)
			},
		)
	})

	testsuite.IsDestroyed(t, server)
}

func TestConn_receiveTarget(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	server, err := NewSocks5Server("test", logger.Test, nil)
	require.NoError(t, err)

	t.Run("failed to read three", func(t *testing.T) {
		conn := &conn{
			server: server,
			local:  testsuite.NewMockConnWithReadError(),
		}

		target := conn.receiveTarget()
		require.Empty(t, target)
	})

	t.Run("invalid version", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(c net.Conn) {
				conn := &conn{
					server: server,
					local:  c,
				}
				target := conn.receiveTarget()
				require.Empty(t, target)
			},
			func(conn net.Conn) {
				req := make([]byte, 4)

				_, err := conn.Write(req)
				require.NoError(t, err)
			},
		)
	})

	t.Run("unknown command", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(c net.Conn) {
				conn := &conn{
					server: server,
					local:  c,
				}
				target := conn.receiveTarget()
				require.Empty(t, target)
			},
			func(conn net.Conn) {
				req := make([]byte, 4)
				req[0] = version5
				req[1] = 0xff

				_, err := conn.Write(req)
				require.NoError(t, err)

				// receive response
				_, err = io.CopyN(ioutil.Discard, conn, 3)
				require.NoError(t, err)
			},
		)
	})

	t.Run("invalid reserved", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(c net.Conn) {
				conn := &conn{
					server: server,
					local:  c,
				}
				target := conn.receiveTarget()
				require.Empty(t, target)
			},
			func(conn net.Conn) {
				req := make([]byte, 4)
				req[0] = version5
				req[1] = connect
				req[2] = 0xff

				_, err := conn.Write(req)
				require.NoError(t, err)

				// receive response
				_, err = io.CopyN(ioutil.Discard, conn, 3)
				require.NoError(t, err)
			},
		)
	})

	t.Run("IPv4", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(c net.Conn) {
				conn := &conn{
					server: server,
					local:  c,
				}
				target := conn.receiveTarget()
				require.Empty(t, target)
			},
			func(conn net.Conn) {
				req := make([]byte, 4+net.IPv4len)
				req[0] = version5
				req[1] = connect
				req[2] = reserve
				req[3] = ipv4

				_, err := conn.Write(req)
				require.NoError(t, err)

				err = conn.Close()
				require.NoError(t, err)
			},
		)
	})

	t.Run("invalid IPv4", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(c net.Conn) {
				conn := &conn{
					server: server,
					local:  c,
				}
				target := conn.receiveTarget()
				require.Empty(t, target)
			},
			func(conn net.Conn) {
				req := make([]byte, 4+net.IPv4len-1)
				req[0] = version5
				req[1] = connect
				req[2] = reserve
				req[3] = ipv4

				_, err := conn.Write(req)
				require.NoError(t, err)

				err = conn.Close()
				require.NoError(t, err)
			},
		)
	})

	testsuite.IsDestroyed(t, server)
}
