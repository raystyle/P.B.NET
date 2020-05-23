package testsuite

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

	t.Run("Close", func(t *testing.T) {
		err := conn.Close()
		require.NoError(t, err)
	})
}

func TestNewMockConnWithReadError(t *testing.T) {
	conn := NewMockConnWithReadError()
	_, err := conn.Read(nil)
	IsMockConnReadError(t, err)
}

func TestNewMockConnWithReadPanic(t *testing.T) {
	defer func() {
		err := errors.New(fmt.Sprint(recover()))
		IsMockConnReadPanic(t, err)
	}()
	conn := NewMockConnWithReadPanic()
	_, _ = conn.Read(nil)
}

func TestNewMockConnWithWriteError(t *testing.T) {
	conn := NewMockConnWithWriteError()
	_, err := conn.Write(nil)
	IsMockConnWriteError(t, err)
}

func TestNewMockConnWithWritePanic(t *testing.T) {
	defer func() {
		err := errors.New(fmt.Sprint(recover()))
		IsMockConnWritePanic(t, err)
	}()
	conn := NewMockConnWithWritePanic()
	_, _ = conn.Write(nil)
}

func TestNewMockConnWithCloseError(t *testing.T) {
	t.Run("after close", func(t *testing.T) {
		conn := NewMockConnWithCloseError()

		err := conn.Close()
		IsMockConnCloseError(t, err)

		_, err = conn.Read(nil)
		IsMockConnClosedError(t, err)

		_, err = conn.Write(nil)
		IsMockConnClosedError(t, err)

		err = conn.SetDeadline(time.Time{})
		IsMockConnClosedError(t, err)

		err = conn.SetReadDeadline(time.Time{})
		IsMockConnClosedError(t, err)

		err = conn.SetWriteDeadline(time.Time{})
		IsMockConnClosedError(t, err)
	})

	t.Run("read", func(t *testing.T) {
		conn := NewMockConnWithCloseError()

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := conn.Read(nil)
			IsMockConnCloseError(t, err)
		}()

		// wait read
		time.Sleep(250 * time.Millisecond)

		err := conn.Close()
		IsMockConnCloseError(t, err)

		wg.Wait()
	})
}

func TestNewMockConnWithClosePanic(t *testing.T) {
	defer func() {
		err := errors.New(fmt.Sprint(recover()))
		IsMockConnClosePanic(t, err)
	}()
	conn := NewMockConnWithClosePanic()
	_ = conn.Close()
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

func TestMockContext(t *testing.T) {
	ctx := new(mockContext)

	t.Run("Deadline", func(t *testing.T) {
		deadline, ok := ctx.Deadline()
		require.Zero(t, deadline)
		require.False(t, ok)
	})

	t.Run("Done", func(t *testing.T) {
		done := ctx.Done()
		require.Nil(t, done)
	})

	t.Run("Err", func(t *testing.T) {
		err := ctx.Err()
		require.NoError(t, err)
	})

	t.Run("Value", func(t *testing.T) {
		val := ctx.Value(nil)
		require.Nil(t, val)
	})
}

func TestNewMockContextWithError(t *testing.T) {
	ctx, cancel := NewMockContextWithError()
	defer cancel()

	done := ctx.Done()
	require.NotNil(t, done)

	err := ctx.Err()
	IsMockContextError(t, err)
}

func TestMockResponseWriter(t *testing.T) {
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

func TestNewMockResponseWriterWithHijackError(t *testing.T) {
	rw := NewMockResponseWriterWithHijackError()

	conn, brw, err := rw.(http.Hijacker).Hijack()
	require.Error(t, err)
	require.Nil(t, brw)
	require.Nil(t, conn)
}

func TestNewMockResponseWriterWithWriteError(t *testing.T) {
	rw := NewMockResponseWriterWithWriteError()

	conn, brw, err := rw.(http.Hijacker).Hijack()
	require.NoError(t, err)
	require.Nil(t, brw)
	require.NotNil(t, conn)

	_, err = conn.Write(nil)
	require.Error(t, err)
}

func TestNewMockResponseWriterWithClosePanic(t *testing.T) {
	rw := NewMockResponseWriterWithClosePanic()

	conn, brw, err := rw.(http.Hijacker).Hijack()
	require.NoError(t, err)
	require.Nil(t, brw)
	require.NotNil(t, conn)

	defer DeferForPanic(t)
	_ = conn.Close()
}
