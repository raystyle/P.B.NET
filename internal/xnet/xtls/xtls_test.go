package xtls

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
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
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
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
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
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
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	clientCfg.ServerName = "localhost"

	server, client := net.Pipe()
	server = Server(server, serverCfg)
	client = Client(client, clientCfg)
	f(t, server, client, false)
}

func TestDialContext_Timeout(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const network = "tcp"
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
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

	require.NoError(t, listener.Close())
	testsuite.IsDestroyed(t, listener)
}

func TestDialContext_Cancel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const network = "tcp"
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
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

	require.NoError(t, listener.Close())
	testsuite.IsDestroyed(t, listener)
}

func TestDialContext_Panic(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const network = "tcp"
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	clientCfg.ServerName = "localhost"

	listener, err := Listen(network, "localhost:0", serverCfg)
	require.NoError(t, err)
	address := listener.Addr().String()
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := listener.Accept()
		require.NoError(t, err)
	}()

	server, client := net.Pipe()
	client = Client(client, clientCfg)
	var pg *monkey.PatchGuard
	patch := func(conn *tls.Conn) error {
		pg.Unpatch()
		_ = conn.Close()
		panic(monkey.Panic)
	}
	pg = monkey.PatchInstanceMethod(client, "Close", patch)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := DialContext(ctx, network, address, clientCfg, 0, nil)
	require.Error(t, err)
	require.Nil(t, conn)

	require.NoError(t, client.Close())
	require.NoError(t, server.Close())

	// TODO [external] go internal bug: *tls.Conn memory leaks
	// testsuite.IsDestroyed(t, client)
	// testsuite.IsDestroyed(t, server)

	require.NoError(t, listener.Close())
	testsuite.IsDestroyed(t, listener)
	wg.Wait()
}

func TestFailedToListenAndDial(t *testing.T) {
	_, err := Listen("udp", "", nil)
	require.Error(t, err)
	_, err = Dial("udp", "", new(tls.Config), 0, nil)
	require.Error(t, err)
}
