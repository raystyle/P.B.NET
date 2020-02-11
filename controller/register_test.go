package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/module/info"
	"project/internal/testsuite"
	"project/internal/xnet"

	"project/node"
)

func TestTrustNodeAndConfirm(t *testing.T) {
	Node := testGenerateInitialNode(t)

	listener, err := Node.GetListener(testInitialNodeListenerTag)
	require.NoError(t, err)
	bListener := &bootstrap.Listener{
		Mode:    xnet.ModeTCP,
		Network: "tcp",
		Address: listener.Addr().String(),
	}

	req, err := ctrl.TrustNode(context.Background(), bListener)
	require.NoError(t, err)
	require.Equal(t, info.GetSystemInfo(), req.SystemInfo)
	t.Log(req.SystemInfo)
	err = ctrl.ConfirmTrustNode(context.Background(), bListener, req)
	require.NoError(t, err)

	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)
}

func testGenerateInitialNodeAndTrust(t testing.TB) *node.Node {
	Node := testGenerateInitialNode(t)

	listener, err := Node.GetListener(testInitialNodeListenerTag)
	require.NoError(t, err)
	bListener := &bootstrap.Listener{
		Mode:    xnet.ModeTCP,
		Network: "tcp",
		Address: listener.Addr().String(),
	}
	// trust node
	req, err := ctrl.TrustNode(context.Background(), bListener)
	require.NoError(t, err)
	err = ctrl.ConfirmTrustNode(context.Background(), bListener, req)
	require.NoError(t, err)
	// connect
	err = ctrl.Synchronize(context.Background(), Node.GUID(), bListener)
	require.NoError(t, err)
	return Node
}
