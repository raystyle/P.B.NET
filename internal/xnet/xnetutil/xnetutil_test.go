package xnetutil

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestCheckPortString(t *testing.T) {
	err := CheckPortString("1234")
	require.NoError(t, err)

	err = CheckPortString("")
	require.Equal(t, ErrEmptyPort, err)

	err = CheckPortString("s")
	require.Error(t, err)
	err = CheckPortString("0")
	require.Error(t, err)
	err = CheckPortString("65536")
	require.Error(t, err)
}

func TestCheckPort(t *testing.T) {
	err := CheckPort(123)
	require.NoError(t, err)

	err = CheckPort(0)
	require.Error(t, err)
	require.Equal(t, "invalid port range: 0", err.Error())

	err = CheckPort(65536)
	require.Error(t, err)
	require.Equal(t, "invalid port range: 65536", err.Error())
}

func TestTrafficUnit_String(t *testing.T) {
	require.Equal(t, "1023 Byte", TrafficUnit(1023).String())
	require.Equal(t, "1.000 KB", TrafficUnit(1024).String())
	require.Equal(t, "1.500 KB", TrafficUnit(1536).String())
	require.Equal(t, "1.000 MB", TrafficUnit(1024*1024).String())
	require.Equal(t, "1.500 MB", TrafficUnit(1536*1024).String())
	require.Equal(t, "1.000 GB", TrafficUnit(1024*1024*1024).String())
	require.Equal(t, "1.500 GB", TrafficUnit(1536*1024*1024).String())
	require.Equal(t, "1.000 TB", TrafficUnit(1024*1024*1024*1024).String())
	require.Equal(t, "1.500 TB", TrafficUnit(1536*1024*1024*1024).String())
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
	_ = client.Close()
	_ = server.Close()
	testsuite.IsDestroyed(t, client)
	testsuite.IsDestroyed(t, server)

	// default deadline
	server, client = net.Pipe()
	client = DeadlineConn(client, 0)
	server = DeadlineConn(server, 0)
	_ = client.Close()
	_ = server.Close()
	testsuite.IsDestroyed(t, client)
	testsuite.IsDestroyed(t, server)
}
