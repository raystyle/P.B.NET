package quic

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

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

func TestConn_Close(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
	defer gm.Compare()

	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	listener, err := Listen("udp4", "localhost:0", serverCfg, 0)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, listener.Close())
		testsuite.IsDestroyed(t, listener)
	}()
	address := listener.Addr().String()

	var server net.Conn
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		server, err = listener.Accept()
		require.NoError(t, err)
	}()
	client, err := Dial("udp4", address, clientCfg, 0)
	require.NoError(t, err)
	wg.Wait()

	wg.Add(8)
	// server
	go func() {
		defer wg.Done()
		_ = server.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = server.Write(testsuite.Bytes())
	}()
	go func() {
		defer wg.Done()
		_ = server.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = server.Write(testsuite.Bytes())
	}()
	// client
	go func() {
		defer wg.Done()
		_ = client.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = client.Write(testsuite.Bytes())
	}()
	go func() {
		defer wg.Done()
		_ = client.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = client.Write(testsuite.Bytes())
	}()
	wg.Wait()

	testsuite.IsDestroyed(t, client)
	testsuite.IsDestroyed(t, server)
}
