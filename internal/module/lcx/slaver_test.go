package lcx

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func testGenerateListenerAndSlaver(t *testing.T) (*Listener, *Slaver) {
	listener := testGenerateListener(t)
	err := listener.Start()
	require.NoError(t, err)
	lNetwork := "tcp"
	lAddress := listener.testIncomeAddress()
	dstNetwork := "tcp"
	dstAddress := "127.0.0.1:" + testsuite.HTTPServerPort
	opts := Options{LocalAddress: "127.0.0.1:0"}
	slaver, err := NewSlaver("test", lNetwork, lAddress,
		dstNetwork, dstAddress, logger.Test, &opts)
	require.NoError(t, err)
	return listener, slaver
}

func TestSlaver(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	listener, slaver := testGenerateListenerAndSlaver(t)
	err := slaver.Start()
	require.NoError(t, err)

	// user dial local listener
	for i := 0; i < 3; i++ {
		lConn, err := net.Dial("tcp", listener.testLocalAddress())
		require.NoError(t, err)
		testsuite.ProxyConn(t, lConn)
	}

	time.Sleep(2 * time.Second)

	t.Log(slaver.Name())
	t.Log(slaver.Info())
	t.Log(slaver.Status())

	err = slaver.Restart()
	require.NoError(t, err)

	slaver.Stop()
	testsuite.IsDestroyed(t, slaver)
	listener.Stop()
	testsuite.IsDestroyed(t, listener)
}
