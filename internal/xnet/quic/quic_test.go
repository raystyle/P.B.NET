package quic

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestListenAndDial(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	if testsuite.IPv4Enabled {
		testListenAndDial(t, "udp4")
	}
	if testsuite.IPv6Enabled {
		testListenAndDial(t, "udp6")
	}
}

func testListenAndDial(t *testing.T, network string) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t, "127.0.0.1")
	listener, err := Listen(network, "localhost:0", serverCfg, time.Second)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		return Dial(network, address, clientCfg.Clone(), time.Second)
	}, true)
}

func TestListenAndDialContext(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	if testsuite.IPv4Enabled {
		testListenAndDialContext(t, "udp4")
	}
	if testsuite.IPv6Enabled {
		testListenAndDialContext(t, "udp6")
	}
}

func testListenAndDialContext(t *testing.T, network string) {
	const timeout = 0

	serverCfg, clientCfg := testsuite.TLSConfigPair(t, "127.0.0.1")
	listener, err := Listen(network, "localhost:0", serverCfg, timeout)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		return DialContext(ctx, network, address, clientCfg.Clone(), timeout)
	}, true)
}

func TestFailedToListen(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		network = "udp"
		timeout = 0
	)

	t.Run("invalid address", func(t *testing.T) {
		_, err := Listen(network, "foo address", nil, timeout)
		require.Error(t, err)
	})

	t.Run("net.ListenUDP", func(t *testing.T) {
		_, err := Listen(network, "0.0.0.1:0", nil, timeout)
		require.Error(t, err)
	})

	t.Run("quic.Listen", func(t *testing.T) {
		patch := func(net.PacketConn, *tls.Config, *quic.Config) (quic.Listener, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(quic.Listen, patch)
		defer pg.Unpatch()

		_, err := Listen(network, "localhost:0", new(tls.Config), timeout)
		monkey.IsMonkeyError(t, err)
	})
}

func TestFailedToAccept(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		network = "udp"
		timeout = 0
	)

	// get *quic.baseServer
	rawConn, err := net.ListenUDP(network, nil)
	require.NoError(t, err)

	serverCfg, _ := testsuite.TLSConfigPair(t, "127.0.0.1")
	quicListener, err := quic.Listen(rawConn, serverCfg.Clone(), nil)
	require.NoError(t, err)

	// patch
	patch := func(interface{}, context.Context) (quic.Session, error) {
		return nil, monkey.Error
	}
	pg := monkey.PatchInstanceMethod(quicListener, "Accept", patch)
	defer pg.Unpatch()

	listener, err := Listen(network, "localhost:0", serverCfg, timeout)
	require.NoError(t, err)
	_, err = listener.Accept()
	monkey.IsMonkeyError(t, err)

	err = listener.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, listener)

	err = quicListener.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, quicListener)

	err = rawConn.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, rawConn)
}

func TestFailedToDialContext(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		network = "udp"
		timeout = 0
	)

	t.Run("invalid address", func(t *testing.T) {
		_, err := Dial(network, "foo address", nil, timeout)
		require.Error(t, err)
	})

	t.Run("net.ListenUDP", func(t *testing.T) {
		patch := func(string, *net.UDPAddr) (*net.UDPConn, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(net.ListenUDP, patch)
		defer pg.Unpatch()

		_, err := Dial(network, "localhost:0", nil, timeout)
		monkey.IsMonkeyError(t, err)
	})

	t.Run("quic.DialContext", func(t *testing.T) {
		_, err := Dial(network, "0.0.0.1:0", new(tls.Config), time.Second)
		require.Error(t, err)
	})

	t.Run("session.OpenStreamSync", func(t *testing.T) {
		serverCfg, clientCfg := testsuite.TLSConfigPair(t, "127.0.0.1")
		listener, err := Listen(network, "localhost:0", serverCfg, timeout)
		require.NoError(t, err)
		address := listener.Addr().String()

		// get *quic.session
		clientCfg.NextProtos = []string{defaultNextProto}
		session, err := quic.DialAddr(address, clientCfg, nil)
		require.NoError(t, err)
		// patch
		patch := func(interface{}, context.Context) (quic.Stream, error) {
			return nil, monkey.Error
		}
		pg := monkey.PatchInstanceMethod(session, "OpenStreamSync", patch)
		defer pg.Unpatch()

		_, err = Dial(network, address, clientCfg, time.Second)
		monkey.IsMonkeyError(t, err)

		err = listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})

	t.Run("stream.Write", func(t *testing.T) {
		serverCfg, clientCfg := testsuite.TLSConfigPair(t, "127.0.0.1")
		listener, err := Listen(network, "localhost:0", serverCfg, timeout)
		require.NoError(t, err)
		address := listener.Addr().String()

		// get *quic.stream
		clientCfg.NextProtos = []string{defaultNextProto}
		session, err := quic.DialAddr(address, clientCfg, nil)
		require.NoError(t, err)
		stream, err := session.OpenStreamSync(context.Background())
		require.NoError(t, err)
		// patch
		patch := func(interface{}, []byte) (int, error) {
			return 0, monkey.Error
		}
		pg := monkey.PatchInstanceMethod(stream, "Write", patch)
		defer pg.Unpatch()

		_, err = Dial(network, address, clientCfg, time.Second)
		monkey.IsMonkeyError(t, err)

		err = listener.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, listener)
	})
}

func TestDialContext(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	conn, err := DialContext(context.Background(), "udp", "", nil, time.Second)
	require.EqualError(t, err, "missing port in address")
	require.Nil(t, conn)
}

func TestConn_Close(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		network = "udp"
		timeout = 0
	)

	serverCfg, clientCfg := testsuite.TLSConfigPair(t, "127.0.0.1")
	listener, err := Listen(network, "localhost:0", serverCfg, timeout)
	require.NoError(t, err)
	address := listener.Addr().String()
	server, client := testsuite.AcceptAndDial(t, listener, func() (net.Conn, error) {
		return Dial(network, address, clientCfg, timeout)
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
	client, err = Dial(network, address, clientCfg, timeout)
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)

	err = listener.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, listener)
}

func TestFailedToAcceptStream(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		network = "udp"
		timeout = 0
	)

	serverCfg, clientCfg := testsuite.TLSConfigPair(t, "127.0.0.1")
	listener, err := Listen(network, "localhost:0", serverCfg, timeout)
	require.NoError(t, err)
	address := listener.Addr().String()

	// client close
	server, client := testsuite.AcceptAndDial(t, listener, func() (net.Conn, error) {
		return Dial(network, address, clientCfg, timeout)
	})

	err = client.Close()
	require.NoError(t, err)

	// make sure connection is closed
	time.Sleep(time.Second)

	err = client.SetDeadline(time.Time{})
	require.Error(t, err)

	err = client.SetWriteDeadline(time.Time{})
	require.Error(t, err)

	buf := make([]byte, 1)
	_, err = server.Read(buf)
	require.Error(t, err)
	_, err = server.Write(buf)
	require.Error(t, err)

	err = server.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
	testsuite.IsDestroyed(t, server)

	// server close
	server, client = testsuite.AcceptAndDial(t, listener, func() (net.Conn, error) {
		return Dial(network, address, clientCfg, timeout)
	})

	err = server.Close()
	require.NoError(t, err)
	_, err = server.Read(buf)
	require.Equal(t, ErrConnClosed, err)

	err = client.Close()
	require.NoError(t, err)
	testsuite.IsDestroyed(t, client)
	testsuite.IsDestroyed(t, server)

	err = listener.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, listener)
}
