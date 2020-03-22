package lcx

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/monkey"
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

	t.Log(slaver.Name())
	t.Log(slaver.Info())
	t.Log(slaver.Status())

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

func TestNewSlaver(t *testing.T) {
	const (
		tag        = "test"
		lNetwork   = "tcp"
		lAddress   = "127.0.0.1:80"
		dstNetwork = "tcp"
		dstAddress = "127.0.0.1:3389"
	)

	t.Run("empty tag", func(t *testing.T) {
		_, err := NewSlaver("", lNetwork, lAddress,
			dstNetwork, dstAddress, nil, nil)
		require.Error(t, err)
	})

	t.Run("invalid listener address", func(t *testing.T) {
		_, err := NewSlaver(tag, "foo", "foo",
			dstNetwork, dstAddress, nil, nil)
		require.Error(t, err)
	})

	t.Run("invalid destination address", func(t *testing.T) {
		_, err := NewSlaver(tag, lNetwork, lAddress,
			"foo", "foo", nil, nil)
		require.Error(t, err)
	})

	t.Run("empty options", func(t *testing.T) {
		_, err := NewSlaver(tag, lNetwork, lAddress,
			dstNetwork, dstAddress, nil, nil)
		require.NoError(t, err)
	})
}

func TestSlaver_Start(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	listener, slaver := testGenerateListenerAndSlaver(t)
	err := slaver.Start()
	require.NoError(t, err)
	err = slaver.Start()
	require.Error(t, err)

	slaver.Stop()
	testsuite.IsDestroyed(t, slaver)
	listener.Stop()
	testsuite.IsDestroyed(t, listener)
}

func TestSlaver_Stop(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	listener, slaver := testGenerateListenerAndSlaver(t)
	err := slaver.Start()
	require.NoError(t, err)

	lConn, err := net.Dial("tcp", listener.testLocalAddress())
	require.NoError(t, err)
	defer func() { _ = lConn.Close() }()

	slaver.Stop()
	slaver.Stop()
	testsuite.IsDestroyed(t, slaver)
	listener.Stop()
	testsuite.IsDestroyed(t, listener)
}

func TestSlaver_serve(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("full", func(t *testing.T) {
		listener, slaver := testGenerateListenerAndSlaver(t)
		slaver.opts.MaxConns = 1 // force change
		err := slaver.Start()
		require.NoError(t, err)

		lConn, err := net.Dial("tcp", listener.testLocalAddress())
		require.NoError(t, err)
		defer func() { _ = lConn.Close() }()

		// wait call full()
		time.Sleep(time.Second)

		slaver.Stop()
		testsuite.IsDestroyed(t, slaver)
		listener.Stop()
		testsuite.IsDestroyed(t, listener)
	})

	t.Run("failed to connect listener", func(t *testing.T) {
		listener, slaver := testGenerateListenerAndSlaver(t)
		slaver.lAddress = "0.0.0.0:1"
		slaver.opts.MaxConns = 1 // force change
		err := slaver.Start()
		require.NoError(t, err)

		// wait serve()
		time.Sleep(time.Second)

		slaver.Stop()
		testsuite.IsDestroyed(t, slaver)
		listener.Stop()
		testsuite.IsDestroyed(t, listener)
	})

	t.Run("panic", func(t *testing.T) {
		listener, slaver := testGenerateListenerAndSlaver(t)

		// patch
		patchFunc := func(context.Context, time.Duration) (context.Context, context.CancelFunc) {
			panic(monkey.Panic)
		}
		pg := monkey.Patch(context.WithTimeout, patchFunc)
		defer pg.Unpatch()

		err := slaver.Start()
		require.NoError(t, err)

		// wait serve()
		time.Sleep(time.Second)

		slaver.Stop()
		testsuite.IsDestroyed(t, slaver)
		listener.Stop()
		testsuite.IsDestroyed(t, listener)
	})
}

func TestSlaver_trackConn(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	listener, slaver := testGenerateListenerAndSlaver(t)

	require.False(t, slaver.trackConn(nil, true))

	slaver.Stop()
	testsuite.IsDestroyed(t, slaver)
	listener.Stop()
	testsuite.IsDestroyed(t, listener)
}

func TestSConn_Serve(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("track conn", func(t *testing.T) {
		listener, slaver := testGenerateListenerAndSlaver(t)

		slaver.ctx = context.Background()
		_, server := net.Pipe()
		conn := slaver.newConn(server)
		conn.Serve()

		slaver.Stop()
		testsuite.IsDestroyed(t, slaver)
		listener.Stop()
		testsuite.IsDestroyed(t, listener)
	})

	t.Run("failed to connect target", func(t *testing.T) {
		listener, slaver := testGenerateListenerAndSlaver(t)
		slaver.dstAddress = "0.0.0.0:1"

		err := slaver.Start()
		require.NoError(t, err)

		// wait serve()
		time.Sleep(time.Second)

		slaver.Stop()
		testsuite.IsDestroyed(t, slaver)
		listener.Stop()
		testsuite.IsDestroyed(t, listener)
	})

	t.Run("local failed read", func(t *testing.T) {
		listener, slaver := testGenerateListenerAndSlaver(t)

		// patch
		dialer := new(net.Dialer)
		patchFunc := func(interface{}, context.Context, string, string) (net.Conn, error) {
			return testsuite.DialMockConnWithWriteError(context.Background(), "", "")
		}
		pg := monkey.PatchInstanceMethod(dialer, "DialContext", patchFunc)
		defer pg.Unpatch()

		err := slaver.Start()
		require.NoError(t, err)

		// wait serve()
		time.Sleep(10 * time.Millisecond)

		slaver.Stop()
		testsuite.IsDestroyed(t, slaver)
		listener.Stop()
		testsuite.IsDestroyed(t, listener)
	})

	t.Run("done block local to remote", func(t *testing.T) {
		listener, slaver := testGenerateListenerAndSlaver(t)

		// patch
		conn := new(sConn)
		patchFunc := func(c *sConn) {
			done := make(chan struct{}, 2)
			// block
			done <- struct{}{}
			done <- struct{}{}
			c.slaver.wg.Add(1)
			go c.serve(done)

			time.Sleep(time.Second)
			go slaver.Stop()
			go listener.Stop()

			<-c.slaver.ctx.Done()
		}
		pg := monkey.PatchInstanceMethod(conn, "Serve", patchFunc)
		defer pg.Unpatch()

		err := slaver.Start()
		require.NoError(t, err)

		lConn, err := net.Dial("tcp", listener.testLocalAddress())
		require.NoError(t, err)
		_, _ = lConn.Write(make([]byte, 1))

		// wait serve
		time.Sleep(time.Second)

		slaver.Stop()
		listener.Stop()

		// because of monkey
		// testsuite.IsDestroyed(t, slaver)
		// testsuite.IsDestroyed(t, listener)
	})

	t.Run("done block remote to local", func(t *testing.T) {
		listener, slaver := testGenerateListenerAndSlaver(t)

		// patch
		conn := new(sConn)
		patchFunc := func(c *sConn) {
			done := make(chan struct{}, 2)
			// block
			done <- struct{}{}
			c.slaver.wg.Add(1)
			go c.serve(done)

			time.Sleep(time.Second)
			go slaver.Stop()
			go listener.Stop()

			<-c.slaver.ctx.Done()
		}
		pg := monkey.PatchInstanceMethod(conn, "Serve", patchFunc)
		defer pg.Unpatch()

		err := slaver.Start()
		require.NoError(t, err)

		lConn, err := net.Dial("tcp", listener.testLocalAddress())
		require.NoError(t, err)
		testsuite.SendHTTPRequest(t, lConn)

		// wait serve
		time.Sleep(time.Second)

		slaver.Stop()
		listener.Stop()

		// because of monkey
		// testsuite.IsDestroyed(t, slaver)
		// testsuite.IsDestroyed(t, listener)
	})

	t.Run("panic from copy", func(t *testing.T) {
		listener, slaver := testGenerateListenerAndSlaver(t)
		err := slaver.Start()
		require.NoError(t, err)

		// patch
		conn := new(net.TCPConn)
		patchFunc := func(interface{}, time.Time) error {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(conn, "SetReadDeadline", patchFunc)
		defer pg.Unpatch()

		// wait serve()
		time.Sleep(time.Second)

		slaver.Stop()
		testsuite.IsDestroyed(t, slaver)
		listener.Stop()
		testsuite.IsDestroyed(t, listener)
	})
}
