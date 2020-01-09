package xnet

import (
	"net"
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

func TestListenAndDial_QUIC(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
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

	cfg := &Config{
		Network:   network,
		Address:   "localhost:0",
		TLSConfig: serverCfg,
	}
	listener, err := Listen(ModeQUIC, cfg)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		cfg := &Config{
			Network:   network,
			Address:   address,
			TLSConfig: clientCfg,
		}
		return Dial(ModeQUIC, cfg)
	}, true)
}

func TestListenAndDial_Light(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
	defer gm.Compare()

	if testsuite.IPv4Enabled {
		testListenAndDialLight(t, "tcp4")
	}
	if testsuite.IPv6Enabled {
		testListenAndDialLight(t, "tcp6")
	}
}

func testListenAndDialLight(t *testing.T, network string) {
	cfg := &Config{
		Network: network,
		Address: "localhost:0",
	}
	listener, err := Listen(ModeLight, cfg)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		cfg := &Config{
			Network: network,
			Address: address,
		}
		return Dial(ModeLight, cfg)
	}, true)
}

func TestListenAndDial_TLS(t *testing.T) {
	gm := testsuite.MarkGoRoutines(t)
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

	cfg := &Config{
		Network:   network,
		Address:   "localhost:0",
		TLSConfig: serverCfg,
	}
	listener, err := Listen(ModeTLS, cfg)
	require.NoError(t, err)
	address := listener.Addr().String()
	testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
		cfg := &Config{
			Network:   network,
			Address:   address,
			TLSConfig: clientCfg,
		}
		return Dial(ModeTLS, cfg)
	}, true)
}

func TestFailedToListenAndDial(t *testing.T) {
	// listen
	listener, err := Listen(ModeQUIC, &Config{Network: "tcp"})
	require.EqualError(t, err, "mismatched mode and network: quic tcp")
	require.Nil(t, listener)

	listener, err = Listen(ModeLight, &Config{Network: "udp"})
	require.EqualError(t, err, "mismatched mode and network: light udp")
	require.Nil(t, listener)

	listener, err = Listen(ModeTLS, &Config{Network: "udp"})
	require.EqualError(t, err, "mismatched mode and network: tls udp")
	require.Nil(t, listener)

	// listen with unknown mode
	listener, err = Listen("foo mode", &Config{Network: "udp"})
	require.EqualError(t, err, "unknown mode: foo mode")
	require.Nil(t, listener)

	// dial
	conn, err := Dial(ModeQUIC, &Config{Network: "tcp"})
	require.EqualError(t, err, "mismatched mode and network: quic tcp")
	require.Nil(t, conn)

	conn, err = Dial(ModeLight, &Config{Network: "udp"})
	require.EqualError(t, err, "mismatched mode and network: light udp")
	require.Nil(t, conn)

	conn, err = Dial(ModeTLS, &Config{Network: "udp"})
	require.EqualError(t, err, "mismatched mode and network: tls udp")
	require.Nil(t, conn)

	// dial with unknown mode
	conn, err = Dial("foo mode", &Config{Network: "udp"})
	require.EqualError(t, err, "unknown mode: foo mode")
	require.Nil(t, conn)
}
