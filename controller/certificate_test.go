package controller

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/guid"
	"project/internal/protocol"
	"project/internal/xnet"
)

func Test_issue_verify_certificate(t *testing.T) {
	init_ctrl(t)
	g := bytes.Repeat([]byte{1}, guid.SIZE)
	node := &bootstrap.Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "localhost:9931",
	}
	cert := ctrl.issue_certificate(g, node)
	// with node guid
	require.True(t, ctrl.verify_certificate(cert, g, node))
	// with controller guid
	require.True(t, ctrl.verify_certificate(cert, protocol.CTRL_GUID, node))
}

func Test_verify_invalid_certificate(t *testing.T) {
	init_ctrl(t)
	g := bytes.Repeat([]byte{1}, guid.SIZE)
	node := &bootstrap.Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "localhost:9931",
	}
	// ----------------------with node guid--------------------------
	// no size
	require.False(t, ctrl.verify_certificate(nil, g, node))
	// invalid size
	cert := []byte{0, 1}
	require.False(t, ctrl.verify_certificate(cert, g, node))
	// invalid certificate
	cert = []byte{0, 1, 0}
	require.False(t, ctrl.verify_certificate(cert, g, node))
	// -------------------with controller guid-----------------------
	// no size
	cert = []byte{0, 1, 0}
	require.False(t, ctrl.verify_certificate(cert, protocol.CTRL_GUID, node))
	// invalid size
	cert = []byte{0, 1, 0, 0, 1}
	require.False(t, ctrl.verify_certificate(cert, protocol.CTRL_GUID, node))
	// invalid certificate
	cert = []byte{0, 1, 0, 0, 1, 0}
	require.False(t, ctrl.verify_certificate(cert, protocol.CTRL_GUID, node))
}
