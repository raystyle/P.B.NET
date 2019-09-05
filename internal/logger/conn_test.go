package logger

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConn(t *testing.T) {
	conn, err := net.Dial("tcp", "github.com:443")
	require.NoError(t, err)
	t.Log(Conn(conn))
	_ = conn.Close()
}
