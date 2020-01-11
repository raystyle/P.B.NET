package quic

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestListenAndDial(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
	defer gm.Compare()

	if testsuite.IPv4Enabled {
		testListenAndDial(t, "udp4")
	}
	if testsuite.IPv6Enabled {
		testListenAndDial(t, "udp6")
	}
}

func testListenAndDial(t *testing.T, network string) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	listener, err := Listen(network, "localhost:0", serverCfg, time.Second)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		return Dial(network, address, clientCfg.Clone(), time.Second)
	}, true)
}

func TestListenAndDialContext(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
	defer gm.Compare()

	if testsuite.IPv4Enabled {
		testListenAndDialContext(t, "udp4")
	}
	if testsuite.IPv6Enabled {
		testListenAndDialContext(t, "udp6")
	}
}

func testListenAndDialContext(t *testing.T, network string) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	listener, err := Listen(network, "localhost:0", serverCfg, 0)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		return DialContext(ctx, network, address, clientCfg.Clone(), 0)
	}, true)
}

func TestFailedToListen(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
	defer gm.Compare()

	t.Run("invalid address", func(t *testing.T) {
		_, err := Listen("udp", "foo address", nil, 0)
		require.Error(t, err)
	})

	t.Run("net.ListenUDP", func(t *testing.T) {
		_, err := Listen("udp", "0.0.0.1:0", nil, 0)
		require.Error(t, err)
	})

	t.Run("quic.Listen", func(t *testing.T) {
		patchFunc := func(net.PacketConn, *tls.Config, *quic.Config) (quic.Listener, error) {
			return nil, errors.New("monkey error")
		}
		pg := testsuite.Patch(quic.Listen, patchFunc)
		defer pg.Unpatch()

		_, err := Listen("udp", "localhost:0", new(tls.Config), 0)
		require.Error(t, err)
	})
}

func TestFailedToAccept(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
	defer gm.Compare()

	// get *quic.baseServer
	rawConn, err := net.ListenUDP("udp", nil)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, rawConn.Close())
		testsuite.IsDestroyed(t, rawConn)
	}()
	serverCfg, _ := testsuite.TLSConfigPair(t)
	quicListener, err := quic.Listen(rawConn, serverCfg.Clone(), nil)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, quicListener.Close())
		testsuite.IsDestroyed(t, quicListener)
	}()
	// patch
	patchFunc := func(interface{}, context.Context) (quic.Session, error) {
		return nil, errors.New("monkey error")
	}
	typ := reflect.TypeOf(quicListener)
	pg := testsuite.PatchInstanceMethod(typ, "Accept", patchFunc)
	defer pg.Unpatch()

	listener, err := Listen("udp", "localhost:0", serverCfg, 0)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, listener.Close())
		testsuite.IsDestroyed(t, listener)
	}()
	_, err = listener.Accept()
	require.Error(t, err)
}

func TestFailedToDialContext(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
	defer gm.Compare()

	t.Run("invalid address", func(t *testing.T) {
		_, err := Dial("udp", "foo address", nil, 0)
		require.Error(t, err)
	})

	t.Run("net.ListenUDP", func(t *testing.T) {
		patchFunc := func(string, *net.UDPAddr) (*net.UDPConn, error) {
			return nil, errors.New("monkey error")
		}
		pg := testsuite.Patch(net.ListenUDP, patchFunc)
		defer pg.Unpatch()

		_, err := Dial("udp", "localhost:0", nil, 0)
		require.Error(t, err)
	})

	t.Run("quic.DialContext", func(t *testing.T) {
		_, err := Dial("udp", "0.0.0.1:0", new(tls.Config), time.Second)
		require.Error(t, err)
	})

	t.Run("session.OpenStreamSync", func(t *testing.T) {
		serverCfg, clientCfg := testsuite.TLSConfigPair(t)
		listener, err := Listen("udp", "localhost:0", serverCfg, 0)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, listener.Close())
			testsuite.IsDestroyed(t, listener)
		}()
		address := listener.Addr().String()

		// get *quic.session
		clientCfg.NextProtos = []string{defaultNextProto}
		session, err := quic.DialAddr(address, clientCfg, nil)
		require.NoError(t, err)
		// patch
		patchFunc := func(interface{}, context.Context) (quic.Stream, error) {
			return nil, errors.New("monkey error")
		}
		typ := reflect.TypeOf(session)
		pg := testsuite.PatchInstanceMethod(typ, "OpenStreamSync", patchFunc)
		defer pg.Unpatch()

		_, err = Dial("udp", address, clientCfg, time.Second)
		require.Error(t, err)
	})

	t.Run("stream.Write", func(t *testing.T) {
		serverCfg, clientCfg := testsuite.TLSConfigPair(t)
		listener, err := Listen("udp", "localhost:0", serverCfg, 0)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, listener.Close())
			testsuite.IsDestroyed(t, listener)
		}()
		address := listener.Addr().String()

		// get *quic.stream
		clientCfg.NextProtos = []string{defaultNextProto}
		session, err := quic.DialAddr(address, clientCfg, nil)
		require.NoError(t, err)
		stream, err := session.OpenStreamSync(context.Background())
		require.NoError(t, err)
		// patch
		patchFunc := func(interface{}, []byte) (int, error) {
			return 0, errors.New("monkey error")
		}
		typ := reflect.TypeOf(stream)
		pg := testsuite.PatchInstanceMethod(typ, "Write", patchFunc)
		defer pg.Unpatch()

		_, err = Dial("udp", address, clientCfg, time.Second)
		require.Error(t, err)
	})
}

func TestConn_Close(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
	defer gm.Compare()

	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	listener, err := Listen("udp", "localhost:0", serverCfg, 0)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, listener.Close())
		testsuite.IsDestroyed(t, listener)
	}()
	address := listener.Addr().String()
	server, client := testsuite.AcceptAndDial(t, listener, func() (conn net.Conn, err error) {
		return Dial("udp", address, clientCfg, 0)
	})

	wg := sync.WaitGroup{}
	writeAndClose := func(conn net.Conn) {
		go func() {
			defer wg.Done()
			_ = conn.Close()
		}()
		go func() {
			defer wg.Done()
			_, _ = conn.Write(testsuite.Bytes())
		}()
	}
	wg.Add(8)
	writeAndClose(server)
	writeAndClose(server)
	writeAndClose(client)
	writeAndClose(client)
	wg.Wait()

	testsuite.IsDestroyed(t, client)
	testsuite.IsDestroyed(t, server)

	// Close() before acceptStream()
	client, err = Dial("udp", address, clientCfg, 0)
	require.NoError(t, err)
	require.NoError(t, client.Close())

	testsuite.IsDestroyed(t, client)
}

func TestFailedToAcceptStream(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
	defer gm.Compare()

	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	listener, err := Listen("udp", "localhost:0", serverCfg, 0)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, listener.Close())
		testsuite.IsDestroyed(t, listener)
	}()
	address := listener.Addr().String()

	// client close
	server, client := testsuite.AcceptAndDial(t, listener, func() (conn net.Conn, err error) {
		return Dial("udp", address, clientCfg, 0)
	})
	require.NoError(t, client.Close())
	require.Error(t, server.SetDeadline(time.Time{}))
	require.Error(t, server.SetWriteDeadline(time.Time{}))
	buf := make([]byte, 1)
	_, err = server.Read(buf)
	require.Error(t, err)
	_, err = server.Write(buf)
	require.Error(t, err)

	require.NoError(t, server.Close())
	testsuite.IsDestroyed(t, client)
	testsuite.IsDestroyed(t, server)

	// server close
	server, client = testsuite.AcceptAndDial(t, listener, func() (conn net.Conn, err error) {
		return Dial("udp", address, clientCfg, 0)
	})
	require.NoError(t, server.Close())
	_, err = server.Read(buf)
	require.Equal(t, ErrConnClosed, err)

	require.NoError(t, client.Close())
	testsuite.IsDestroyed(t, client)
	testsuite.IsDestroyed(t, server)
}
