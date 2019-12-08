package controller

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/xnet"
)

func TestSender_Connect(t *testing.T) {
	testInitCtrl(t)
	NODE := testGenerateNode(t)
	defer NODE.Exit(nil)
	listener, err := NODE.GetListener(testListenerTag)
	require.NoError(t, err)
	node := bootstrap.Node{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: listener.Addr().String(),
	}
	// trust node
	req, err := ctrl.TrustNode(context.Background(), &node)
	require.NoError(t, err)
	err = ctrl.ConfirmTrustNode(context.Background(), &node, req)
	require.NoError(t, err)
	// connect
	err = ctrl.sender.Connect(&node, NODE.GUID())
	require.NoError(t, err)
	// disconnect
	guid := strings.ToUpper(hex.EncodeToString(NODE.GUID()))
	err = ctrl.sender.Disconnect(guid)
	require.NoError(t, err)
}

func TestSender_Broadcast(t *testing.T) {

}
