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

	// get node information
	listener := testGetNodeListener(t, Node, testInitialNodeListenerTag)
	nnr, err := ctrl.TrustNode(context.Background(), listener)
	require.NoError(t, err)
	require.Equal(t, info.GetSystemInfo(), nnr.SystemInfo)
	spew.Dump(nnr)

	// confirm
	reply := ReplyNodeRegister{
		ID:   nnr.ID,
		Zone: "test",
	}
	err = ctrl.ConfirmTrustNode(context.Background(), &reply)
	require.NoError(t, err)

	// can't call ctrl.DeleteNodeUnscoped(nodeGUID), because controller
	// doesn't connect any Nodes.
	err = ctrl.database.DeleteNodeUnscoped(nodeGUID)
	require.NoError(t, err)

	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)
}
