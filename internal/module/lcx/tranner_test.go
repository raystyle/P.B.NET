package lcx

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestTranner(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	destNetwork := "tcp"
	destAddress := "127.0.0.1:" + testsuite.HTTPServerPort
	opts := Options{LocalAddress: "127.0.0.1:0"}
	tranner, err := NewTranner("test", destNetwork, destAddress, logger.Test, &opts)
	require.NoError(t, err)

	// test connect test http server
	require.Equal(t, "", tranner.testAddress())
	conn, err := net.Dial("tcp", tranner.testAddress())
	require.NoError(t, err)
	testsuite.ProxyConn(t, conn)

	t.Log(tranner.Name())
	t.Log(tranner.Info())
	t.Log(tranner.Status())

	tranner.Stop()
	testsuite.IsDestroyed(t, tranner)
}
