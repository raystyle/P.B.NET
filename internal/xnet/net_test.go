package xnet

import (
	"crypto/tls"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestCheckModeNetwork(t *testing.T) {
	err := CheckModeNetwork(ModeQUIC, "udp")
	require.NoError(t, err)
	err = CheckModeNetwork(ModeLight, "tcp")
	require.NoError(t, err)
	err = CheckModeNetwork(ModeTLS, "tcp")
	require.NoError(t, err)

	err = CheckModeNetwork(ModeQUIC, "tcp")
	require.EqualError(t, err, "mismatched mode and network: quic tcp")
	err = CheckModeNetwork(ModeLight, "udp")
	require.EqualError(t, err, "mismatched mode and network: light udp")
	err = CheckModeNetwork(ModeTLS, "udp")
	require.EqualError(t, err, "mismatched mode and network: tls udp")

	err = CheckModeNetwork("", "")
	require.Equal(t, ErrEmptyMode, err)
	err = CheckModeNetwork(ModeTLS, "")
	require.Equal(t, ErrEmptyNetwork, err)

	err = CheckModeNetwork("foo mode", "foo network")
	require.EqualError(t, err, "unknown mode: foo mode")
}

func TestListener(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	listener, err := Listen(ModeLight, "tcp", "localhost:0", nil)
	require.NoError(t, err)

	t.Run("AcceptEx and Mode()", func(t *testing.T) {
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := listener.AcceptEx()
			require.NoError(t, err)
			err = conn.Conn.(testsuite.Handshake).Handshake()
			require.NoError(t, err)
			err = conn.Close()
			require.NoError(t, err)
		}()
		conn, err := Dial(ModeLight, "tcp", listener.Addr().String(), nil)
		require.NoError(t, err)
		err = conn.Close()
		require.NoError(t, err)
		wg.Wait()

		require.Equal(t, ModeLight, listener.Mode())
	})

	t.Run("failed to AcceptEx", func(t *testing.T) {
		// patch
		var tcpListener *net.TCPListener
		patchFunc := func(*net.TCPListener) (net.Conn, error) {
			return nil, testsuite.ErrMonkey
		}
		pg := testsuite.PatchInstanceMethod(tcpListener, "Accept", patchFunc)
		defer pg.Unpatch()

		_, err = listener.AcceptEx()
		testsuite.IsMonkeyError(t, err)
	})

	require.NoError(t, listener.Close())
	testsuite.IsDestroyed(t, listener)
}

func TestListenAndDial_QUIC(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	if testsuite.IPv4Enabled {
		testListenAndDialQUIC(t, "udp4")
	}
	if testsuite.IPv6Enabled {
		testListenAndDialQUIC(t, "udp6")
	}
}

func testListenAndDialQUIC(t *testing.T, network string) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	clientCfg.ServerName = "localhost"

	opts := &Options{TLSConfig: serverCfg}
	listener, err := Listen(ModeQUIC, network, "localhost:0", opts)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		opts := &Options{TLSConfig: clientCfg.Clone()}
		return Dial(ModeQUIC, network, address, opts)
	}, true)
}

func TestListenAndDial_Light(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	if testsuite.IPv4Enabled {
		testListenAndDialLight(t, "tcp4")
	}
	if testsuite.IPv6Enabled {
		testListenAndDialLight(t, "tcp6")
	}
}

func testListenAndDialLight(t *testing.T, network string) {
	listener, err := Listen(ModeLight, network, "localhost:0", nil)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		return Dial(ModeLight, network, address, nil)
	}, true)
}

func TestListenAndDial_TLS(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	if testsuite.IPv4Enabled {
		testListenAndDialTLS(t, "tcp4")
	}
	if testsuite.IPv6Enabled {
		testListenAndDialTLS(t, "tcp6")
	}
}

func testListenAndDialTLS(t *testing.T, network string) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	clientCfg.ServerName = "localhost"

	opts := &Options{TLSConfig: serverCfg}
	listener, err := Listen(ModeTLS, network, "localhost:0", opts)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		opts := &Options{TLSConfig: clientCfg.Clone()}
		return Dial(ModeTLS, network, address, opts)
	}, true)
}

func TestFailedToListenAndDial(t *testing.T) {
	t.Run("failed to Listen-Check", func(t *testing.T) {
		listener, err := Listen(ModeTLS, "udp", "", nil)
		require.EqualError(t, err, "mismatched mode and network: tls udp")
		require.Nil(t, listener)
	})

	t.Run("failed to Listen", func(t *testing.T) {
		listener, err := Listen(ModeTLS, "tcp", "", nil)
		require.Error(t, err)
		require.Nil(t, listener)
	})

	t.Run("failed to Dial-Check", func(t *testing.T) {
		opts := Options{TLSConfig: new(tls.Config)}
		conn, err := Dial(ModeTLS, "udp", "", &opts)
		require.EqualError(t, err, "mismatched mode and network: tls udp")
		require.Nil(t, conn)
	})

	t.Run("failed to Dial", func(t *testing.T) {
		opts := Options{TLSConfig: new(tls.Config)}
		conn, err := Dial(ModeTLS, "tcp", "", &opts)
		require.Error(t, err)
		require.Nil(t, conn)
	})
}
