package xnet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckModeNetwork(t *testing.T) {
	err := CheckModeNetwork(TLS, "tcp")
	require.NoError(t, err)
	err = CheckModeNetwork(QUIC, "udp")
	require.NoError(t, err)
	err = CheckModeNetwork(Light, "tcp")
	require.NoError(t, err)

	err = CheckModeNetwork(TLS, "udp")
	require.Error(t, err)
	require.Equal(t, "mismatched mode and network: tls udp", err.Error())
	err = CheckModeNetwork(QUIC, "tcp")
	require.Error(t, err)
	require.Equal(t, "mismatched mode and network: quic tcp", err.Error())
	err = CheckModeNetwork(Light, "udp")
	require.Error(t, err)
	require.Equal(t, "mismatched mode and network: light udp", err.Error())

	err = CheckModeNetwork("", "")
	require.Equal(t, ErrEmptyMode, err)
	err = CheckModeNetwork(TLS, "")
	require.Equal(t, ErrEmptyNetwork, err)

	err = CheckModeNetwork("xxxx", "xxxx")
	require.Error(t, err)
	require.Equal(t, "unknown mode: xxxx", err.Error())
}
