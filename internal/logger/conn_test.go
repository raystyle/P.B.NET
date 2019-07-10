package logger

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Conn(t *testing.T) {
	conn, err := net.Dial("tcp", "github.com:443")
	require.Nil(t, err, err)
	b := Conn(conn)
	t.Log(b)
}
