package lcx

import (
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

func testGenerateListener(t *testing.T) *Listener {
	iNetwork := "tcp"
	iAddress := "127.0.0.1:0"
	opts := Options{LocalAddress: "127.0.0.1:0"}
	listener, err := NewListener("test", iNetwork, iAddress, logger.Test, &opts)
	require.NoError(t, err)
	return listener
}

func TestListener(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	listener := testGenerateListener(t)

	t.Log(listener.Name())
	t.Log(listener.Info())
	t.Log(listener.Status())

	require.Zero(t, listener.testIncomeAddress())
	require.Zero(t, listener.testLocalAddress())

	err := listener.Start()
	require.NoError(t, err)

	// mock slaver and start copy
	hConn, err := net.Dial("tcp", "127.0.0.1:"+testsuite.HTTPServerPort)
	require.NoError(t, err)
	// must close
	defer func() { _ = hConn.Close() }()
	iConn, err := net.Dial("tcp", listener.testIncomeAddress())
	require.NoError(t, err)
	// not close
	go func() {
		_, _ = io.Copy(hConn, iConn)
	}()
	go func() {
		_, _ = io.Copy(iConn, hConn)
	}()
	// user dial local listener
	lConn, err := net.Dial("tcp", listener.testLocalAddress())
	require.NoError(t, err)
	testsuite.ProxyConn(t, lConn)

	t.Log(listener.Name())
	t.Log(listener.Info())
	t.Log(listener.Status())

	err = listener.Restart()
	require.NoError(t, err)

	listener.Stop()
	testsuite.IsDestroyed(t, listener)
}

func TestNewListener(t *testing.T) {
	const (
		tag      = "test"
		iNetwork = "tcp"
		iAddress = "127.0.0.1:80"
	)

	t.Run("empty tag", func(t *testing.T) {
		_, err := NewListener("", "", "", nil, nil)
		require.Error(t, err)
	})

	t.Run("invalid income address", func(t *testing.T) {
		_, err := NewListener(tag, "foo", "foo", nil, nil)
		require.Error(t, err)
	})

	t.Run("empty options", func(t *testing.T) {
		_, err := NewListener(tag, iNetwork, iAddress, logger.Test, nil)
		require.NoError(t, err)
	})

	t.Run("invalid local address", func(t *testing.T) {
		opts := Options{
			LocalNetwork: "foo",
			LocalAddress: "foo",
		}
		_, err := NewListener(tag, iNetwork, iAddress, logger.Test, &opts)
		require.Error(t, err)
	})
}

func TestListener_Start(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("start twice", func(t *testing.T) {
		listener := testGenerateListener(t)

		err := listener.Start()
		require.NoError(t, err)
		err = listener.Start()
		require.Error(t, err)

		listener.Stop()
		testsuite.IsDestroyed(t, listener)
	})

	t.Run("failed to listen income", func(t *testing.T) {
		iNetwork := "tcp"
		iAddress := "0.0.0.1:0"
		tranner, err := NewListener("test", iNetwork, iAddress, logger.Test, nil)
		require.NoError(t, err)

		err = tranner.Start()
		require.Error(t, err)

		tranner.Stop()
		testsuite.IsDestroyed(t, tranner)
	})

	t.Run("failed to listen local", func(t *testing.T) {
		iNetwork := "tcp"
		iAddress := "127.0.0.1:0"
		opts := Options{LocalAddress: "0.0.0.1:0"}
		tranner, err := NewListener("test", iNetwork, iAddress, logger.Test, &opts)
		require.NoError(t, err)

		err = tranner.Start()
		require.Error(t, err)

		tranner.Stop()
		testsuite.IsDestroyed(t, tranner)
	})
}

func TestListener_Stop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	listener := testGenerateListener(t)
	err := listener.Start()
	require.NoError(t, err)

	iConn, err := net.Dial("tcp", listener.testIncomeAddress())
	require.NoError(t, err)
	defer func() { _ = iConn.Close() }()

	lConn, err := net.Dial("tcp", listener.testLocalAddress())
	require.NoError(t, err)
	defer func() { _ = lConn.Close() }()

	// wait serve
	time.Sleep(time.Second)

	t.Log(listener.Status())

	listener.Stop()
	listener.Stop()
	testsuite.IsDestroyed(t, listener)
}

func TestListener_serve(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("accept income", func(t *testing.T) {
		listener := testGenerateListener(t)
		err := listener.Start()
		require.NoError(t, err)

		iConn, err := net.Dial("tcp", listener.testIncomeAddress())
		require.NoError(t, err)
		defer func() { _ = iConn.Close() }()

		// wait accept
		time.Sleep(time.Second)

		listener.Stop()
		testsuite.IsDestroyed(t, listener)
	})

	t.Run("panic", func(t *testing.T) {
		patch := func(net.Listener, int) net.Listener {
			return testsuite.NewMockListenerWithAcceptPanic()
		}
		pg := monkey.Patch(netutil.LimitListener, patch)
		defer pg.Unpatch()

		listener := testGenerateListener(t)
		err := listener.Start()
		require.NoError(t, err)

		listener.Stop()
		testsuite.IsDestroyed(t, listener)
	})
}

func TestListener_accept(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	patch := func(net.Listener, int) net.Listener {
		return testsuite.NewMockListenerWithAcceptError()
	}
	pg := monkey.Patch(netutil.LimitListener, patch)
	defer pg.Unpatch()

	listener := testGenerateListener(t)
	err := listener.Start()
	require.NoError(t, err)

	listener.Stop()
	testsuite.IsDestroyed(t, listener)
}

func TestListener_trackConn(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	listener := testGenerateListener(t)

	require.False(t, listener.trackConn(nil, true))

	testsuite.IsDestroyed(t, listener)
}

func TestLConn_Serve(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("track conn", func(t *testing.T) {
		listener := testGenerateListener(t)

		client, server := net.Pipe()
		conn := listener.newConn(client, server)
		conn.Serve()

		testsuite.IsDestroyed(t, listener)
	})

	t.Run("panic from copy", func(t *testing.T) {
		listener := testGenerateListener(t)

		err := listener.Start()
		require.NoError(t, err)

		patch := func(io.Writer, io.Reader) (int64, error) {
			panic(monkey.Panic)
		}
		pg := monkey.Patch(io.Copy, patch)
		defer pg.Unpatch()

		iConn, err := net.Dial("tcp", listener.testIncomeAddress())
		require.NoError(t, err)
		defer func() { _ = iConn.Close() }()

		lConn, err := net.Dial("tcp", listener.testLocalAddress())
		require.NoError(t, err)
		defer func() { _ = lConn.Close() }()

		// wait serve
		time.Sleep(time.Second)

		listener.Stop()
		testsuite.IsDestroyed(t, listener)
	})
}
