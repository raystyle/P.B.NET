package lcx

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/netutil"

	"project/internal/logger"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func testGenerateTranner(t *testing.T) *Tranner {
	dstNetwork := "tcp"
	dstAddress := "127.0.0.1:" + testsuite.HTTPServerPort
	opts := Options{LocalAddress: "127.0.0.1:0"}
	tranner, err := NewTranner("test", dstNetwork, dstAddress, logger.Test, &opts)
	require.NoError(t, err)
	return tranner
}

func TestTranner(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	tranner := testGenerateTranner(t)
	require.Equal(t, "", tranner.testAddress())
	err := tranner.Start()
	require.NoError(t, err)

	// test connect test http server
	for i := 0; i < 3; i++ {
		conn, err := net.Dial("tcp", tranner.testAddress())
		require.NoError(t, err)
		testsuite.ProxyConn(t, conn)
	}

	t.Log(tranner.Name())
	t.Log(tranner.Info())
	t.Log(tranner.Status())

	err = tranner.Restart()
	require.NoError(t, err)

	tranner.Stop()
	testsuite.IsDestroyed(t, tranner)
}

func TestNewTranner(t *testing.T) {
	const (
		tag      = "test"
		dNetwork = "tcp"
		dAddress = "127.0.0.1:80"
	)

	t.Run("empty tag", func(t *testing.T) {
		_, err := NewTranner("", "", "", nil, nil)
		require.Error(t, err)
	})

	t.Run("invalid destination address", func(t *testing.T) {
		_, err := NewTranner(tag, "foo", "foo", nil, nil)
		require.Error(t, err)
	})

	t.Run("empty options", func(t *testing.T) {
		_, err := NewTranner(tag, dNetwork, dAddress, logger.Test, nil)
		require.NoError(t, err)
	})

	t.Run("invalid local address", func(t *testing.T) {
		opts := Options{
			LocalNetwork: "foo",
			LocalAddress: "foo",
		}
		_, err := NewTranner(tag, dNetwork, dAddress, logger.Test, &opts)
		require.Error(t, err)
	})
}

func TestTranner_Start(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("start twice", func(t *testing.T) {
		tranner := testGenerateTranner(t)

		err := tranner.Start()
		require.NoError(t, err)
		err = tranner.Start()
		require.Error(t, err)

		tranner.Stop()
		testsuite.IsDestroyed(t, tranner)
	})

	t.Run("failed to listen", func(t *testing.T) {
		dstNetwork := "tcp"
		dstAddress := "127.0.0.1:" + testsuite.HTTPServerPort
		opts := Options{LocalAddress: "0.0.0.1:0"}
		tranner, err := NewTranner("test", dstNetwork, dstAddress, logger.Test, &opts)
		require.NoError(t, err)

		err = tranner.Start()
		require.Error(t, err)

		tranner.Stop()
		testsuite.IsDestroyed(t, tranner)
	})
}

func TestTranner_Stop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	tranner := testGenerateTranner(t)
	err := tranner.Start()
	require.NoError(t, err)

	// force close tranner
	conn, err := net.Dial("tcp", tranner.testAddress())
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	// wait tran
	time.Sleep(time.Second)

	t.Log(tranner.Status())

	tranner.Stop()
	tranner.Stop()
	testsuite.IsDestroyed(t, tranner)
}

func TestTranner_serve(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("panic", func(t *testing.T) {
		// patch
		listener := netutil.LimitListener(nil, 1)
		patchFunc := func(interface{}) (net.Conn, error) {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(listener, "Accept", patchFunc)
		defer pg.Unpatch()

		tranner := testGenerateTranner(t)
		err := tranner.Start()
		require.NoError(t, err)

		tranner.Stop()
		testsuite.IsDestroyed(t, tranner)
	})

	t.Run("failed to accept", func(t *testing.T) {
		// patch
		patchFunc := func(net.Listener, int) net.Listener {
			return testsuite.NewMockListenerWithError()
		}
		pg := monkey.Patch(netutil.LimitListener, patchFunc)
		defer pg.Unpatch()

		tranner := testGenerateTranner(t)
		err := tranner.Start()
		require.NoError(t, err)

		tranner.wg.Wait()

		tranner.Stop()
		testsuite.IsDestroyed(t, tranner)
	})
}

func TestTranner_trackConn(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	tranner := testGenerateTranner(t)

	require.False(t, tranner.trackConn(nil, true))

	testsuite.IsDestroyed(t, tranner)
}

func TestTConn_Serve(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("closed", func(t *testing.T) {
		tranner := testGenerateTranner(t)

		_, server := net.Pipe()
		conn := tranner.newConn(server)
		conn.Serve()

		testsuite.IsDestroyed(t, tranner)
	})

	t.Run("failed to connect target", func(t *testing.T) {
		dstNetwork := "tcp"
		dstAddress := "0.0.0.0:1"
		opts := Options{LocalAddress: "127.0.0.1:0"}
		tranner, err := NewTranner("test", dstNetwork, dstAddress, logger.Test, &opts)
		require.NoError(t, err)

		err = tranner.Start()
		require.NoError(t, err)

		conn, err := net.Dial("tcp", tranner.testAddress())
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		// wait tran
		time.Sleep(time.Second)

		tranner.Stop()
		testsuite.IsDestroyed(t, tranner)
	})

	t.Run("panic", func(t *testing.T) {
		tranner := testGenerateTranner(t)
		err := tranner.Start()
		require.NoError(t, err)

		// patch
		patchFunc := func(context.Context, time.Duration) (context.Context, context.CancelFunc) {
			panic(monkey.Panic)
		}
		pg := monkey.Patch(context.WithTimeout, patchFunc)
		defer pg.Unpatch()

		conn, err := net.Dial("tcp", tranner.testAddress())
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		// wait tran
		time.Sleep(time.Second)

		tranner.Stop()
		testsuite.IsDestroyed(t, tranner)
	})

	t.Run("panic from copy", func(t *testing.T) {
		tranner := testGenerateTranner(t)

		err := tranner.Start()
		require.NoError(t, err)

		// patch
		patchFunc := func(io.Writer, io.Reader) (int64, error) {
			panic(monkey.Panic)
		}
		pg := monkey.Patch(io.Copy, patchFunc)
		defer pg.Unpatch()

		conn, err := net.Dial("tcp", tranner.testAddress())
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		// wait tran
		time.Sleep(time.Second)

		tranner.Stop()
		testsuite.IsDestroyed(t, tranner)
	})
}
