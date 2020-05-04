package testsuite

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
)

func TestMockNetError(t *testing.T) {
	err := new(mockNetError)
	require.NotZero(t, err.Error())
	require.False(t, err.Timeout())
	require.False(t, err.Temporary())
}

func TestMockConnLocalAddr(t *testing.T) {
	addr := new(mockConnLocalAddr)
	require.NotZero(t, addr.Network())
	require.NotZero(t, addr.String())
}

func TestMockConnRemoteAddr(t *testing.T) {
	addr := new(mockConnRemoteAddr)
	require.NotZero(t, addr.Network())
	require.NotZero(t, addr.String())
}

func TestMockConn(t *testing.T) {
	conn := new(mockConn)

	t.Run("Read", func(t *testing.T) {
		n, err := conn.Read(nil)
		require.NoError(t, err)
		require.Zero(t, n)
	})

	t.Run("Write", func(t *testing.T) {
		n, err := conn.Write(nil)
		require.NoError(t, err)
		require.Zero(t, n)
	})

	t.Run("Close", func(t *testing.T) {
		err := conn.Close()
		require.NoError(t, err)
	})

	t.Run("LocalAddr", func(t *testing.T) {
		l := mockConnLocalAddr{}
		addr := conn.LocalAddr()
		require.Equal(t, l, addr)
	})

	t.Run("RemoteAddr", func(t *testing.T) {
		r := mockConnRemoteAddr{}
		addr := conn.RemoteAddr()
		require.Equal(t, r, addr)
	})

	t.Run("SetDeadline", func(t *testing.T) {
		err := conn.SetDeadline(time.Time{})
		require.NoError(t, err)
	})

	t.Run("SetReadDeadline", func(t *testing.T) {
		err := conn.SetReadDeadline(time.Time{})
		require.NoError(t, err)
	})

	t.Run("SetWriteDeadline", func(t *testing.T) {
		err := conn.SetWriteDeadline(time.Time{})
		require.NoError(t, err)
	})
}

func TestNewMockConnWithCloseError(t *testing.T) {
	conn := NewMockConnWithCloseError()
	err := conn.Close()
	IsMockConnCloseError(t, err)
}

func TestNewMockConnWithSetDeadlinePanic(t *testing.T) {
	defer func() {
		err := errors.New(fmt.Sprint(recover()))
		IsMockConnSetDeadlinePanic(t, err)
	}()
	conn := NewMockConnWithSetDeadlinePanic()
	_ = conn.SetDeadline(time.Time{})
}

func TestNewMockConnWithSetReadDeadlinePanic(t *testing.T) {
	defer func() {
		err := errors.New(fmt.Sprint(recover()))
		IsMockConnSetReadDeadlinePanic(t, err)
	}()
	conn := NewMockConnWithSetReadDeadlinePanic()
	_ = conn.SetReadDeadline(time.Time{})
}

func TestNewMockConnWithSetWriteDeadlinePanic(t *testing.T) {
	defer func() {
		err := errors.New(fmt.Sprint(recover()))
		IsMockConnSetWriteDeadlinePanic(t, err)
	}()
	conn := NewMockConnWithSetWriteDeadlinePanic()
	_ = conn.SetWriteDeadline(time.Time{})
}

func TestMockListenerAddr(t *testing.T) {
	addr := new(mockListenerAddr)
	require.NotZero(t, addr.Network())
	require.NotZero(t, addr.String())
}

func TestMockListener(t *testing.T) {
	listener := new(mockListener)

	t.Run("Accept", func(t *testing.T) {
		listener := new(mockListener)
		addContextToMockListener(listener, false)

		conn, err := listener.Accept()
		require.NoError(t, err)
		require.Nil(t, conn)

		err = listener.Close()
		require.NoError(t, err)
	})

	t.Run("Addr", func(t *testing.T) {
		a := mockListenerAddr{}
		addr := listener.Addr()
		require.Equal(t, a, addr)
	})
}

func TestNewMockListenerWithAcceptError(t *testing.T) {
	listener := NewMockListenerWithAcceptError()

	for i := 0; i < mockListenerAcceptTimes+1; i++ {
		conn, err := listener.Accept()
		require.Error(t, err)
		require.Nil(t, conn)
	}

	conn, err := listener.Accept()
	IsMockListenerAcceptFatal(t, err)
	require.Nil(t, conn)

	err = listener.Close()
	require.NoError(t, err)
}

func TestNewMockListenerWithAcceptPanic(t *testing.T) {
	defer func() {
		err := errors.New(fmt.Sprint(recover()))
		IsMockListenerAcceptPanic(t, err)
	}()
	listener := NewMockListenerWithAcceptPanic()
	_, _ = listener.Accept()
}

func TestNewMockListenerWithCloseError(t *testing.T) {
	listener := NewMockListenerWithCloseError()

	err := listener.Close()
	IsMockListenerCloseError(t, err)

	conn, err := listener.Accept()
	IsMockListenerClosedError(t, err)
	require.Nil(t, conn)
}

func TestMockResponseWriter(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	rw := NewMockResponseWriter()

	t.Run("Header", func(t *testing.T) {
		require.Nil(t, rw.Header())
	})

	t.Run("Write", func(t *testing.T) {
		n, err := rw.Write(nil)
		require.NoError(t, err)
		require.Equal(t, 0, n)
	})

	t.Run("WriteHeader", func(t *testing.T) {
		rw.WriteHeader(0)
	})

	t.Run("Hijack", func(t *testing.T) {
		conn, rw, err := rw.(http.Hijacker).Hijack()
		require.NoError(t, err)
		require.Nil(t, rw)
		require.NotNil(t, conn)

		err = conn.Close()
		require.NoError(t, err)
	})
}

func TestNewMockResponseWriterWithFailedToHijack(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	rw := NewMockResponseWriterWithFailedToHijack()

	conn, brw, err := rw.(http.Hijacker).Hijack()
	require.Error(t, err)
	require.Nil(t, brw)
	require.Nil(t, conn)
}

func TestNewMockResponseWriterWithFailedToWrite(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	rw := NewMockResponseWriterWithFailedToWrite()

	conn, brw, err := rw.(http.Hijacker).Hijack()
	require.NoError(t, err)
	require.Nil(t, brw)
	require.NotNil(t, conn)

	_, err = conn.Write(nil)
	require.Error(t, err)
}

func TestNewMockResponseWriterWithClosePanic(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	rw := NewMockResponseWriterWithClosePanic()

	conn, brw, err := rw.(http.Hijacker).Hijack()
	require.NoError(t, err)
	require.Nil(t, brw)
	require.NotNil(t, conn)

	defer func() { require.NotNil(t, recover()) }()
	_ = conn.Close()
}

func TestDialMockConnWithReadPanic(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	conn, err := DialMockConnWithReadPanic(context.Background(), "", "")
	require.NoError(t, err)

	defer func() {
		require.NotNil(t, recover())
		err = conn.Close()
		require.NoError(t, err)
	}()
	_, _ = conn.Read(nil)
}

func TestDialMockConnWithWriteError(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	conn, err := DialMockConnWithWriteError(context.Background(), "", "")
	require.NoError(t, err)

	_, err = conn.Read(make([]byte, 1))
	require.NoError(t, err)

	_, err = conn.Write(nil)
	monkey.IsMonkeyError(t, err)

	err = conn.Close()
	require.NoError(t, err)
}

func TestNewMockReadCloserWithReadError(t *testing.T) {
	rc := NewMockReadCloserWithReadError()

	_, err := rc.Read(nil)
	IsMockReadCloserError(t, err)
}

func TestNewMockReadCloserWithReadPanic(t *testing.T) {
	rc := NewMockReadCloserWithReadPanic()

	t.Run("panic", func(t *testing.T) {
		defer func() { require.NotNil(t, recover()) }()
		_, _ = rc.Read(nil)
	})

	t.Run("read after close", func(t *testing.T) {
		err := rc.Close()
		require.NoError(t, err)
		_, err = rc.Read(nil)
		IsMockReadCloserError(t, err)
	})
}
