package xnet

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestCheckModeNetwork(t *testing.T) {
	err := CheckModeNetwork(ModeTLS, "tcp")
	require.NoError(t, err)
	err = CheckModeNetwork(ModeQUIC, "udp")
	require.NoError(t, err)
	err = CheckModeNetwork(ModeLight, "tcp")
	require.NoError(t, err)

	err = CheckModeNetwork(ModeTLS, "udp")
	require.EqualError(t, err, "mismatched mode and network: tls udp")
	err = CheckModeNetwork(ModeQUIC, "tcp")
	require.EqualError(t, err, "mismatched mode and network: quic tcp")
	err = CheckModeNetwork(ModeLight, "udp")
	require.EqualError(t, err, "mismatched mode and network: light udp")

	err = CheckModeNetwork("", "")
	require.Equal(t, ErrEmptyMode, err)
	err = CheckModeNetwork(ModeTLS, "")
	require.Equal(t, ErrEmptyNetwork, err)

	err = CheckModeNetwork("foo mode", "foo network")
	require.EqualError(t, err, "unknown mode: foo mode")
}

func TestListenAndDial_TLS(t *testing.T) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	if testsuite.EnableIPv4() {
		sCfg := &Config{
			Network:   "tcp4",
			Address:   "localhost:0",
			TLSConfig: serverCfg,
		}
		listener, err := Listen(ModeTLS, sCfg)
		require.NoError(t, err)
		addr := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			cCfg := &Config{
				Network:   "tcp4",
				Address:   addr,
				TLSConfig: clientCfg,
			}
			return Dial(ModeTLS, cCfg)
		}, true)
	}

	if testsuite.EnableIPv6() {
		sCfg := &Config{
			Network:   "tcp6",
			Address:   "localhost:0",
			TLSConfig: serverCfg,
		}
		listener, err := Listen(ModeTLS, sCfg)
		require.NoError(t, err)
		addr := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			cCfg := &Config{
				Network:   "tcp6",
				Address:   addr,
				TLSConfig: clientCfg,
			}
			return Dial(ModeTLS, cCfg)
		}, true)
	}
}

func TestListenAndDial_QUIC(t *testing.T) {
	serverCfg, clientCfg := testsuite.TLSConfigPair(t)
	if testsuite.EnableIPv4() {
		sCfg := &Config{
			Network:   "udp4",
			Address:   "localhost:0",
			TLSConfig: serverCfg,
		}
		listener, err := Listen(ModeQUIC, sCfg)
		require.NoError(t, err)
		addr := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			cCfg := &Config{
				Network:   "udp4",
				Address:   addr,
				TLSConfig: clientCfg,
			}
			return Dial(ModeQUIC, cCfg)
		}, true)
	}

	if testsuite.EnableIPv6() {
		sCfg := &Config{
			Network:   "udp6",
			Address:   "localhost:0",
			TLSConfig: serverCfg,
		}
		listener, err := Listen(ModeQUIC, sCfg)
		require.NoError(t, err)
		addr := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			cCfg := &Config{
				Network:   "udp6",
				Address:   addr,
				TLSConfig: clientCfg,
			}
			return Dial(ModeQUIC, cCfg)
		}, true)
	}
}

func TestListenAndDial_Light(t *testing.T) {
	if testsuite.EnableIPv4() {
		sCfg := &Config{
			Network: "tcp4",
			Address: "localhost:0",
		}
		listener, err := Listen(ModeLight, sCfg)
		require.NoError(t, err)
		addr := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			cCfg := &Config{
				Network: "tcp4",
				Address: addr,
			}
			return Dial(ModeLight, cCfg)
		}, true)
	}

	if testsuite.EnableIPv6() {
		sCfg := &Config{
			Network: "tcp6",
			Address: "localhost:0",
		}
		listener, err := Listen(ModeLight, sCfg)
		require.NoError(t, err)
		addr := listener.Addr().String()
		testsuite.ListenerAndDial(t, listener, func() (net.Conn, error) {
			cCfg := &Config{
				Network: "tcp6",
				Address: addr,
			}
			return Dial(ModeLight, cCfg)
		}, true)
	}
}

func TestListenAndDial_Failed(t *testing.T) {
	// listen
	listener, err := Listen(ModeTLS, &Config{Network: "udp"})
	require.EqualError(t, err, "mismatched mode and network: tls udp")
	require.Nil(t, listener)

	listener, err = Listen(ModeQUIC, &Config{Network: "tcp"})
	require.EqualError(t, err, "mismatched mode and network: quic tcp")
	require.Nil(t, listener)

	listener, err = Listen(ModeLight, &Config{Network: "udp"})
	require.EqualError(t, err, "mismatched mode and network: light udp")
	require.Nil(t, listener)

	// listen with unknown mode
	listener, err = Listen("foo mode", &Config{Network: "udp"})
	require.EqualError(t, err, "unknown mode: foo mode")
	require.Nil(t, listener)

	// dial
	conn, err := Dial(ModeTLS, &Config{Network: "udp"})
	require.EqualError(t, err, "mismatched mode and network: tls udp")
	require.Nil(t, conn)

	conn, err = Dial(ModeQUIC, &Config{Network: "tcp"})
	require.EqualError(t, err, "mismatched mode and network: quic tcp")
	require.Nil(t, conn)

	conn, err = Dial(ModeLight, &Config{Network: "udp"})
	require.EqualError(t, err, "mismatched mode and network: light udp")
	require.Nil(t, conn)

	// dial with unknown mode
	conn, err = Dial("foo mode", &Config{Network: "udp"})
	require.EqualError(t, err, "unknown mode: foo mode")
	require.Nil(t, conn)
}
