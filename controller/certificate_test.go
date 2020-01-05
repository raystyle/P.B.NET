package controller

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/guid"
	"project/internal/modules/info"
	"project/internal/xnet"
)

func TestIssueVerifyCertificate(t *testing.T) {
	testInitializeController(t)
	const address = "localhost:9931"
	nodeGUID := bytes.Repeat([]byte{1}, guid.Size)
	cert := ctrl.issueCertificate(address, nodeGUID)
	client := client{ctx: ctrl}
	require.True(t, client.verifyCertificate(cert, address, nodeGUID))
}

func TestVerifyInvalidCertificate(t *testing.T) {
	testInitializeController(t)
	client := client{ctx: ctrl}
	require.False(t, client.verifyCertificate(nil, "foo", []byte{1}))
}

func TestTrustNodeAndConfirm(t *testing.T) {
	testInitializeController(t)
	NODE := testGenerateInitialNode(t)
	defer NODE.Exit(nil)
	listener, err := NODE.GetListener(testInitialNodeListenerTag)
	require.NoError(t, err)
	node := &bootstrap.Node{
		Mode:    xnet.ModeQUIC,
		Network: "udp",
		Address: listener.Addr().String(),
	}
	req, err := ctrl.TrustNode(context.Background(), node)
	require.NoError(t, err)
	require.Equal(t, info.GetSystemInfo(), req.SystemInfo)
	t.Log(req.SystemInfo)
	err = ctrl.ConfirmTrustNode(context.Background(), node, req)
	require.NoError(t, err)
}
