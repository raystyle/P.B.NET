package internal

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

func TestDeadlineConn(t *testing.T) {
	server, client := net.Pipe()
	client = DeadlineConn(client, 100*time.Millisecond)
	server = DeadlineConn(server, 100*time.Millisecond)
	// read timeout
	buf := make([]byte, 1024)
	_, err := client.Read(buf)
	require.Error(t, err)
	_, err = client.Write(buf)
	require.Error(t, err)
	_ = client.Close()
	_ = server.Close()
	testutil.IsDestroyed(t, client, 1)
	testutil.IsDestroyed(t, server, 1)

	// default timeout
	server, client = net.Pipe()
	client = DeadlineConn(client, 0)
	server = DeadlineConn(server, 0)
	_ = client.Close()
	_ = server.Close()
	testutil.IsDestroyed(t, client, 1)
	testutil.IsDestroyed(t, server, 1)
}
