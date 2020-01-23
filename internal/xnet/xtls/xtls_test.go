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
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	listener, err := Listen(network, "localhost:0", serverCfg, 0)
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
	listener, err := Listen(network, "localhost:0", serverCfg, 0)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		return DialContext(ctx, network, address, clientCfg.Clone(), 0, nil)
	}, true)
}

func TestConnWithBackground(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	testConnWithBackground(t, testsuite.ConnSC)
	testConnWithBackground(t, testsuite.ConnCS)
}

func testConnWithBackground(t *testing.T, f func(testing.TB, net.Conn, net.Conn, bool)) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	clientCfg.ServerName = "localhost"

	server, client := net.Pipe()
	server = Server(context.Background(), server, serverCfg, 0)
	client = Client(context.Background(), client, clientCfg, 0)
	f(t, server, client, false)
}

func TestConnWithCancel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	testConnWithCancel(t, testsuite.ConnSC)
	testConnWithCancel(t, testsuite.ConnCS)
}

func testConnWithCancel(t *testing.T, f func(testing.TB, net.Conn, net.Conn, bool)) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	clientCfg.ServerName = "localhost"

	server, client := net.Pipe()
	sCtx, sCancel := context.WithCancel(context.Background())
	defer sCancel()
	server = Server(sCtx, server, serverCfg, 0)
	cCtx, cCancel := context.WithCancel(context.Background())
	defer cCancel()
	client = Client(cCtx, client, clientCfg, 0)
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
	listener, err := Listen(network, "localhost:0", serverCfg, 0)
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

	listener, err := Listen(network, "localhost:0", serverCfg, 0)
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

func TestFailedToListenAndDial(t *testing.T) {
	_, err := Listen("udp", "", nil, 0)
	require.Error(t, err)
	_, err = Dial("udp", "", new(tls.Config), 0, nil)
	require.Error(t, err)
}
