package controller

import (
	"context"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/module/info"
	"project/internal/testsuite"
)

func TestTrustNodeAndConfirm(t *testing.T) {
	Node := testGenerateInitialNode(t)
	nodeGUID := Node.GUID()

	listener := testGetNodeListener(t, Node, testInitialNodeListenerTag)
	req, err := ctrl.TrustNode(context.Background(), listener)
	require.NoError(t, err)
	require.Equal(t, info.GetSystemInfo(), req.SystemInfo)
	spew.Dump(req)
	err = ctrl.ConfirmTrustNode(context.Background(), listener, req)
	require.NoError(t, err)

	// clean
	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)

	err = ctrl.DeleteNodeUnscoped(nodeGUID)
	require.NoError(t, err)
}
