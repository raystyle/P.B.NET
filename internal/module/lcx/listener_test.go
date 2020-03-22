package lcx

import (
	"io"
	"net"
	"testing"

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
	require.Equal(t, "", listener.testIncomeAddress())
	require.Equal(t, "", listener.testLocalAddress())
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
		dNetwork = "tcp"
		dAddress = "127.0.0.1:80"
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
		_, err := NewListener(tag, dNetwork, dAddress, logger.Test, nil)
		require.NoError(t, err)
	})

	t.Run("invalid local address", func(t *testing.T) {
		opts := Options{
			LocalNetwork: "foo",
			LocalAddress: "foo",
		}
		_, err := NewListener(tag, dNetwork, dAddress, logger.Test, &opts)
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

	t.Log(listener.Status())

	listener.Stop()
	listener.Stop()
	testsuite.IsDestroyed(t, listener)
}

func TestListener_serve(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	// patch
	patchFunc := func(net.Listener, int) net.Listener {
		return testsuite.NewMockListenerWithPanic()
	}
	pg := monkey.Patch(netutil.LimitListener, patchFunc)
	defer pg.Unpatch()

	listener := testGenerateListener(t)
	err := listener.Start()
	require.NoError(t, err)

	listener.Stop()
	testsuite.IsDestroyed(t, listener)
}

func TestListener_accept(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	// patch
	patchFunc := func(net.Listener, int) net.Listener {
		return testsuite.NewMockListenerWithError()
	}
	pg := monkey.Patch(netutil.LimitListener, patchFunc)
	defer pg.Unpatch()

	listener := testGenerateListener(t)
	err := listener.Start()
	require.NoError(t, err)

	listener.Stop()
	testsuite.IsDestroyed(t, listener)
}
