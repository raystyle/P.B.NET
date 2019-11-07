package xnet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckModeNetwork(t *testing.T) {
	err := CheckModeNetwork(ModeTLS, "tcp")
	require.NoError(t, err)
	err = CheckModeNetwork(ModeQUIC, "udp")
	require.NoError(t, err)
	err = CheckModeNetwork(ModeLight, "tcp")
	require.NoError(t, err)

	err = CheckModeNetwork(ModeTLS, "udp")
	require.Error(t, err)
	require.Equal(t, "mismatched mode and network: tls udp", err.Error())
	err = CheckModeNetwork(ModeQUIC, "tcp")
	require.Error(t, err)
	require.Equal(t, "mismatched mode and network: quic tcp", err.Error())
	err = CheckModeNetwork(ModeLight, "udp")
	require.Error(t, err)
	require.Equal(t, "mismatched mode and network: light udp", err.Error())

	err = CheckModeNetwork("", "")
	require.Equal(t, ErrEmptyMode, err)
	err = CheckModeNetwork(ModeTLS, "")
	require.Equal(t, ErrEmptyNetwork, err)

	err = CheckModeNetwork("foo mode", "foo network")
	require.Error(t, err)
	require.Equal(t, "unknown mode: foo mode", err.Error())
}

func TestListenAndDial_TLS(t *testing.T) {

}

func TestListenAndDial_QUIC(t *testing.T) {

}

func TestListenAndDial_Light(t *testing.T) {

}
