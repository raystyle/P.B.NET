package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/module/info"
	"project/internal/xnet"
)

func TestTrustNodeAndConfirm(t *testing.T) {
	testInitializeController(t)

	NODE := testGenerateInitialNode(t)
	defer NODE.Exit(nil)

	listener, err := NODE.GetListener(testInitialNodeListenerTag)
	require.NoError(t, err)
	node := &bootstrap.Node{
		Mode:    xnet.ModeTCP,
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
