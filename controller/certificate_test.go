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

func TestIssueVerifyCertificate(t *testing.T) {
	initCtrl(t)
	g := bytes.Repeat([]byte{1}, guid.SIZE)
	node := &bootstrap.Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "localhost:9931",
	}
	cert := ctrl.issueCertificate(node, g)
	// with node guid
	require.True(t, ctrl.verifyCertificate(cert, node, g))
	// with controller guid
	require.True(t, ctrl.verifyCertificate(cert, node, protocol.CtrlGUID))
}

func TestVerifyInvalidCertificate(t *testing.T) {
	initCtrl(t)
	g := bytes.Repeat([]byte{1}, guid.SIZE)
	node := &bootstrap.Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "localhost:9931",
	}
	// ----------------------with node guid--------------------------
	// no size
	require.False(t, ctrl.verifyCertificate(nil, node, g))
	// invalid size
	cert := []byte{0, 1}
	require.False(t, ctrl.verifyCertificate(cert, node, g))
	// invalid certificate
	cert = []byte{0, 1, 0}
	require.False(t, ctrl.verifyCertificate(cert, node, g))
	// -------------------with controller guid-----------------------
	// no size
	cert = []byte{0, 1, 0}
	require.False(t, ctrl.verifyCertificate(cert, node, protocol.CtrlGUID))
	// invalid size
	cert = []byte{0, 1, 0, 0, 1}
	require.False(t, ctrl.verifyCertificate(cert, node, protocol.CtrlGUID))
	// invalid certificate
	cert = []byte{0, 1, 0, 0, 1, 0}
	require.False(t, ctrl.verifyCertificate(cert, node, protocol.CtrlGUID))
}
