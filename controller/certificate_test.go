package controller

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/guid"
	"project/internal/protocol"
)

func TestIssueVerifyCertificate(t *testing.T) {
	const address = "localhost:9931"
	initCtrl(t)
	g := bytes.Repeat([]byte{1}, guid.SIZE)
	cert := ctrl.issueCertificate(address, g)
	// with node guid
	require.True(t, ctrl.verifyCertificate(cert, address, g))
	// with controller guid
	require.True(t, ctrl.verifyCertificate(cert, address, protocol.CtrlGUID))
}

func TestVerifyInvalidCertificate(t *testing.T) {
	const address = "localhost:9931"
	initCtrl(t)
	g := bytes.Repeat([]byte{1}, guid.SIZE)
	// ----------------------with node guid--------------------------
	// no size
	require.False(t, ctrl.verifyCertificate(nil, address, g))
	// invalid size
	cert := []byte{0, 1}
	require.False(t, ctrl.verifyCertificate(cert, address, g))
	// invalid certificate
	cert = []byte{0, 1, 0}
	require.False(t, ctrl.verifyCertificate(cert, address, g))
	// -------------------with controller guid-----------------------
	// no size
	cert = []byte{0, 1, 0}
	require.False(t, ctrl.verifyCertificate(cert, address, protocol.CtrlGUID))
	// invalid size
	cert = []byte{0, 1, 0, 0, 1}
	require.False(t, ctrl.verifyCertificate(cert, address, protocol.CtrlGUID))
	// invalid certificate
	cert = []byte{0, 1, 0, 0, 1, 0}
	require.False(t, ctrl.verifyCertificate(cert, address, protocol.CtrlGUID))
}
