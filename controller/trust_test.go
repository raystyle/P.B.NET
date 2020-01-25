package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/module/info"
	"project/internal/xnet"
)

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
