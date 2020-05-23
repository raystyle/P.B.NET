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
	})

	t.Run("don't support hostname", func(t *testing.T) {
		client := Client{disableExt: true}

		err := client.connectSocks4(nil, "host", 1)
		require.Error(t, err)
	})

	t.Run("hostname too long", func(t *testing.T) {
		client := Client{}
		hostname := strings.Repeat("a", 257)

		err := client.connectSocks4(nil, hostname, 1)
		require.Error(t, err)
	})

	t.Run("write to request", func(t *testing.T) {
		client := Client{}
		conn := testsuite.NewMockConnWithWriteError()

		err := client.connectSocks4(conn, "host", 1)
		require.Error(t, err)
	})

	t.Run("invalid reply", func(t *testing.T) {
		client := Client{}
		srv, cli := net.Pipe()

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
	})

	t.Run("ok", func(t *testing.T) {
		client := Client{}
		srv, cli := net.Pipe()

		go func() {
			_, err := io.CopyN(ioutil.Discard, srv, 9)
			require.NoError(t, err)

			_, err = srv.Write(v4ReplySucceeded)
			require.NoError(t, err)
		}()

		err := client.connectSocks4(cli, "1.1.1.1", 1)
		require.NoError(t, err)
	})
}
