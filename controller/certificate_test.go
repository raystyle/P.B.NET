package controller

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/guid"
	"project/internal/modules/info"
	"project/internal/protocol"
	"project/internal/xnet"
)

func TestIssueVerifyCertificate(t *testing.T) {
	const address = "localhost:9931"
	testInitCtrl(t)
	g := bytes.Repeat([]byte{1}, guid.Size)
	cert := ctrl.issueCertificate(address, g)
	// with node guid
	require.True(t, ctrl.verifyCertificate(cert, address, g))
	// with controller guid
	require.True(t, ctrl.verifyCertificate(cert, address, protocol.CtrlGUID))
}

func TestVerifyInvalidCertificate(t *testing.T) {
	const address = "localhost:9931"
	testInitCtrl(t)
	g := bytes.Repeat([]byte{1}, guid.Size)
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

func TestTrustNodeAndConfirm(t *testing.T) {
	testInitCtrl(t)
	NODE := testGenerateNode(t)
	defer NODE.Exit(nil)
	listener, err := NODE.GetListener(testListenerTag)
	require.NoError(t, err)
	node := &bootstrap.Node{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: listener.Addr().String(),
	}
	req, err := ctrl.TrustNode(context.Background(), node)
	require.NoError(t, err)
	require.Equal(t, info.GetSystemInfo(), req.SystemInfo)
	t.Log(req.SystemInfo)
	err = ctrl.ConfirmTrustNode(context.Background(), node, req)
	require.NoError(t, err)
}
