package controller

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/xnet"
)

func TestNewSClient(t *testing.T) {
	testInitCtrl(t)
	NODE := testGenerateNode(t, true)
	defer NODE.Exit(nil)
	node := &bootstrap.Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "localhost:62300",
	}
	req, err := ctrl.TrustNode(node)
	require.NoError(t, err)
	err = ctrl.ConfirmTrustNode(node, req)
	require.NoError(t, err)
	sClient, err := newSenderClient(ctrl, node, NODE.TestGetGUID())
	require.NoError(t, err)
	sClient.Close()
}
