package xnetutil

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckPort(t *testing.T) {
	err := CheckPort(123)
	require.NoError(t, err)

	// invalid port
	err = CheckPort(-1)
	require.EqualError(t, err, "invalid port: -1")
	err = CheckPort(65536)
	require.EqualError(t, err, "invalid port: 65536")
}

func TestCheckPortString(t *testing.T) {
	err := CheckPortString("1234")
	require.NoError(t, err)

	err = CheckPortString("")
	require.Equal(t, ErrEmptyPort, err)

	// NaN
	err = CheckPortString("s")
	require.Error(t, err)

	// invalid port
	err = CheckPortString("-1")
	require.Error(t, err)
	err = CheckPortString("65536")
	require.Error(t, err)
}

func TestIPEnabled(t *testing.T) {
	t.Log(IPEnabled())
}

func TestDeadlineConn(t *testing.T) {
	server, client := net.Pipe()
	client = DeadlineConn(client, 100*time.Millisecond)
	server = DeadlineConn(server, 100*time.Millisecond)

	// deadline
	buf := make([]byte, 1024)
	_, err := client.Read(buf)
	require.Error(t, err)
	_, err = client.Write(buf)
	require.Error(t, err)
	err = client.Close()
	require.NoError(t, err)
	err = server.Close()
	require.NoError(t, err)

	// default deadline
	server, client = net.Pipe()
	client = DeadlineConn(client, 0)
	server = DeadlineConn(server, 0)
	err = client.Close()
	require.NoError(t, err)
	err = server.Close()
	require.NoError(t, err)
}
