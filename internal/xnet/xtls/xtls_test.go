package xtls

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestListenAndDial(t *testing.T) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	if testsuite.IPv4Enabled {
		listener, err := Listen("tcp4", "localhost:0", serverCfg.Clone(), 0)
		require.NoError(t, err)
		address := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			return Dial("tcp4", address, clientCfg.Clone(), 0, nil)
		}, true)
	}

	if testsuite.IPv6Enabled {
		listener, err := Listen("tcp6", "localhost:0", serverCfg.Clone(), 0)
		require.NoError(t, err)
		address := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			return Dial("tcp6", address, clientCfg.Clone(), 0, nil)
		}, true)
	}
}

func TestListenAndDialContext(t *testing.T) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	if testsuite.IPv4Enabled {
		listener, err := Listen("tcp4", "localhost:0", serverCfg, 0)
		require.NoError(t, err)
		address := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			return DialContext(ctx, "tcp4", address, clientCfg, 0, nil)
		}, true)
	}

	if testsuite.IPv6Enabled {
		listener, err := Listen("tcp6", "localhost:0", serverCfg, 0)
		require.NoError(t, err)
		address := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			return DialContext(ctx, "tcp6", address, clientCfg, 0, nil)
		}, true)
	}
}

func TestConnWithBackground(t *testing.T) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	clientCfg.ServerName = "localhost"

	server, client := net.Pipe()
	server = Server(context.Background(), server, serverCfg, 0)
	client = Client(context.Background(), client, clientCfg, 0)
	testsuite.ConnSC(t, server, client, false)
	testsuite.IsDestroyed(t, server)
	testsuite.IsDestroyed(t, client)

	server, client = net.Pipe()
	server = Server(context.Background(), server, serverCfg, 0)
	client = Client(context.Background(), client, clientCfg, 0)
	testsuite.ConnCS(t, server, client, false)
	testsuite.IsDestroyed(t, server)
	testsuite.IsDestroyed(t, client)
}

func TestConnSCWithContext(t *testing.T) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	clientCfg.ServerName = "localhost"

	server, client := net.Pipe()
	sCtx, sCancel := context.WithCancel(context.Background())
	defer sCancel()
	server = Server(sCtx, server, serverCfg, 0)
	cCtx, cCancel := context.WithCancel(context.Background())
	defer cCancel()
	client = Client(cCtx, client, clientCfg, 0)
	testsuite.ConnSC(t, server, client, false)
}

func TestConnCSWithContext(t *testing.T) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	clientCfg.ServerName = "localhost"

	server, client := net.Pipe()
	sCtx, sCancel := context.WithCancel(context.Background())
	defer sCancel()
	server = Server(sCtx, server, serverCfg, 0)
	cCtx, cCancel := context.WithCancel(context.Background())
	defer cCancel()
	client = Client(cCtx, client, clientCfg, 0)
	testsuite.ConnCS(t, client, server, false)
}
