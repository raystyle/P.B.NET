package xtls

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestListenAndDial(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	if testsuite.IPv4Enabled {
		testListenAndDial(t, "tcp4")
	}
	if testsuite.IPv6Enabled {
		testListenAndDial(t, "tcp6")
	}
}

func testListenAndDial(t *testing.T, network string) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t, "127.0.0.1")
	listener, err := Listen(network, "localhost:0", serverCfg)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		return Dial(network, address, clientCfg.Clone(), 0, nil)
	}, true)
}

func TestListenAndDialContext(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	if testsuite.IPv4Enabled {
		testListenAndDialContext(t, "tcp4")
	}
	if testsuite.IPv6Enabled {
		testListenAndDialContext(t, "tcp6")
	}
}

func testListenAndDialContext(t *testing.T, network string) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t, "127.0.0.1")
	listener, err := Listen(network, "localhost:0", serverCfg)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		return DialContext(ctx, network, address, clientCfg.Clone(), 0, nil)
	}, true)
}

func TestConn(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	testConn(t, testsuite.ConnSC)
	testConn(t, testsuite.ConnCS)
}

func testConn(t *testing.T, f func(*testing.T, net.Conn, net.Conn, bool)) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t, "127.0.0.1")
	clientCfg.ServerName = "localhost"

	server, client := net.Pipe()
	server = Server(server, serverCfg)
	client = Client(client, clientCfg)
	f(t, server, client, false)
}

func TestDialContext_TLSError(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	conn, err := DialContext(context.Background(), "tcp", "", nil, time.Second, nil)
	require.EqualError(t, err, "missing port in address")
	require.Nil(t, conn)
}

func TestDialContext_Timeout(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const network = "tcp"
	serverCfg, clientCfg := testsuite.TLSConfigPair(t, "127.0.0.1")
	clientCfg.ServerName = "localhost"

	// failed to dialContext
	address := "0.0.0.1:0"
	_, err := Dial(network, address, clientCfg.Clone(), time.Second, nil)
	require.Error(t, err)

	// handshake timeout
	listener, err := Listen(network, "localhost:0", serverCfg)
	require.NoError(t, err)
	address = listener.Addr().String()

	_, err = Dial(network, address, clientCfg.Clone(), time.Second, nil)
	require.Error(t, err)

	err = listener.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, listener)
}

func TestDialContext_Cancel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const network = "tcp"
	serverCfg, clientCfg := testsuite.TLSConfigPair(t, "127.0.0.1")
	clientCfg.ServerName = "localhost"

	listener, err := Listen(network, "localhost:0", serverCfg)
	require.NoError(t, err)
	address := listener.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(time.Second)
		cancel()
	}()
	_, err = DialContext(ctx, network, address, clientCfg, 0, nil)
	require.Error(t, err)

	wg.Wait()

	err = listener.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, listener)
}

func TestDialContext_Panic(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		network = "tcp"
		address = "127.0.0.1:1"
		timeout = 10 * time.Second
	)
	tlsConfig := new(tls.Config)

	t.Run("context error", func(t *testing.T) {
		ctx, cancel := testsuite.NewMockContextWithError()
		defer cancel()
		dialContext := func(context.Context, string, string) (net.Conn, error) {
			return testsuite.NewMockConnWithWriteError(), nil
		}

		_, err := DialContext(ctx, network, address, tlsConfig, timeout, dialContext)
		testsuite.IsMockContextError(t, err)
	})

	t.Run("panic from conn write", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		dialContext := func(context.Context, string, string) (net.Conn, error) {
			return testsuite.NewMockConnWithWritePanic(), nil
		}

		_, err := DialContext(ctx, network, address, tlsConfig, timeout, dialContext)
		testsuite.IsMockConnWritePanic(t, err)
	})
}

func TestFailedToListenAndDial(t *testing.T) {
	_, err := Listen("udp", "", nil)
	require.Error(t, err)

	_, err = Dial("udp", "", new(tls.Config), 0, nil)
	require.Error(t, err)
}
