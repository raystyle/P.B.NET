package controller

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/xnet"
)

func TestSender_Connect(t *testing.T) {
	const address = "localhost:62300"
	testInitCtrl(t)
	NODE := testGenerateNode(t, true)
	defer NODE.Exit(nil)
	node := bootstrap.Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: address,
	}
	// trust node
	req, err := ctrl.TrustNode(&node)
	require.NoError(t, err)
	err = ctrl.ConfirmTrustNode(&node, req)
	require.NoError(t, err)
	// connect
	err = ctrl.sender.Connect(&node, NODE.TestGetGUID())
	require.NoError(t, err)
	// disconnect
	guid := hex.EncodeToString(NODE.TestGetGUID())
	err = ctrl.sender.Disconnect(guid)
	require.NoError(t, err)
}

func TestSender_Broadcast(t *testing.T) {

}
