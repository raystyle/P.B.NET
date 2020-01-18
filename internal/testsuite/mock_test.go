package testsuite

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMockListener_Accept(t *testing.T) {
	listener := new(mockListener)
	conn, err := listener.Accept()
	require.Nil(t, conn)
	require.Nil(t, err)
}

func TestMockResponseWriter(t *testing.T) {
	rw := new(mockResponseWriter)
	require.Nil(t, rw.Header())
	n, err := rw.Write(nil)
	require.Equal(t, 0, n)
	require.Nil(t, err)
	rw.WriteHeader(0)
}
