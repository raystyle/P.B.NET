package virtualconn

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/guid"
	"project/internal/testsuite"
)

const (
	localAddr  byte = 1
	localPort       = 1999
	remoteAddr byte = 2
	remotePort      = 20160507
)

func testGenerateConn() *Conn {
	localGUID := guid.GUID{}
	copy(localGUID[:], bytes.Repeat([]byte{localAddr}, guid.Size))
	localPort := uint32(localPort)
	remoteGUID := guid.GUID{}
	copy(remoteGUID[:], bytes.Repeat([]byte{remoteAddr}, guid.Size))
	remotePort := uint32(remotePort)
	return NewConn(nil, nil, &localGUID, localPort, &remoteGUID, remotePort)
}

func TestListener_WithTimeout(t *testing.T) {
	addr := guid.GUID{}
	copy(addr[:], bytes.Repeat([]byte{localAddr}, guid.Size))

	t.Run("Accept", func(t *testing.T) {
		listener := NewListener(&addr, localPort, time.Second)

		gConn := testGenerateConn()
		err := listener.addConn(gConn)
		require.NoError(t, err)

		aConn, err := listener.Accept()
		require.NoError(t, err)

		require.Equal(t, gConn, aConn.(*Conn))

		err = listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})

	t.Run("Failed to Accept", func(t *testing.T) {
		listener := NewListener(&addr, localPort, time.Second)

		conn, err := listener.Accept()
		require.Error(t, err)
		require.Nil(t, conn)

		err = listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})

	t.Run("AcceptVC", func(t *testing.T) {
		listener := NewListener(&addr, localPort, time.Second)

		gConn := testGenerateConn()
		err := listener.addConn(gConn)
		require.NoError(t, err)

		aConn, err := listener.AcceptVC()
		require.NoError(t, err)

		require.Equal(t, gConn, aConn)

		err = listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})

	t.Run("Failed to AcceptVC", func(t *testing.T) {
		listener := NewListener(&addr, localPort, time.Second)

		conn, err := listener.AcceptVC()
		require.Error(t, err)
		require.Nil(t, conn)

		err = listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})

	t.Run("Addr", func(t *testing.T) {
		listener := NewListener(&addr, localPort, time.Second)

		t.Log("address:", listener.Addr())

		err := listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})

	t.Run("Close()", func(t *testing.T) {
		listener := NewListener(&addr, localPort, time.Second)

		err := listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})
}

func TestListener_WithoutTimeout(t *testing.T) {
	addr := guid.GUID{}
	copy(addr[:], bytes.Repeat([]byte{localAddr}, guid.Size))

	t.Run("Accept", func(t *testing.T) {
		listener := NewListener(&addr, localPort, 0)

		gConn := testGenerateConn()
		err := listener.addConn(gConn)
		require.NoError(t, err)

		aConn, err := listener.Accept()
		require.NoError(t, err)

		require.Equal(t, gConn, aConn.(*Conn))

		err = listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})

	t.Run("Failed to Accept", func(t *testing.T) {
		listener := NewListener(&addr, localPort, 0)

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()

			time.Sleep(time.Second)

			err := listener.Close()
			require.NoError(t, err)
		}()

		conn, err := listener.Accept()
		require.Error(t, err)
		require.Nil(t, conn)

		wg.Wait()

		err = listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})

	t.Run("AcceptVC", func(t *testing.T) {
		listener := NewListener(&addr, localPort, 0)

		gConn := testGenerateConn()
		err := listener.addConn(gConn)
		require.NoError(t, err)

		aConn, err := listener.AcceptVC()
		require.NoError(t, err)

		require.Equal(t, gConn, aConn)

		err = listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})

	t.Run("Failed to AcceptVC", func(t *testing.T) {
		listener := NewListener(&addr, localPort, 0)

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()

			time.Sleep(time.Second)

			err := listener.Close()
			require.NoError(t, err)
		}()

		conn, err := listener.AcceptVC()
		require.Error(t, err)
		require.Nil(t, conn)

		wg.Wait()

		err = listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})

	t.Run("Addr", func(t *testing.T) {
		listener := NewListener(&addr, localPort, 0)

		t.Log("address:", listener.Addr())

		err := listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})

	t.Run("Close()", func(t *testing.T) {
		listener := NewListener(&addr, localPort, 0)

		err := listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})
}

func TestListener_addConn(t *testing.T) {
	addr := guid.GUID{}
	copy(addr[:], bytes.Repeat([]byte{localAddr}, guid.Size))
	listener := NewListener(&addr, localPort, 0)

	listener.conns = nil

	t.Run("timeout", func(t *testing.T) {
		err := listener.addConn(testGenerateConn())
		require.Error(t, err)
	})

	t.Run("closed", func(t *testing.T) {
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()

			time.Sleep(time.Second)

			err := listener.Close()
			require.NoError(t, err)
		}()

		err := listener.addConn(testGenerateConn())
		require.Error(t, err)

		wg.Wait()
	})

	err := listener.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, listener)
}
