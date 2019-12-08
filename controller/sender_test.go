package controller

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/xnet"
	"project/node"
)

func testGenerateNodeAndTrust(t testing.TB) *node.Node {
	testInitCtrl(t)
	NODE := testGenerateNode(t)
	listener, err := NODE.GetListener(testListenerTag)
	require.NoError(t, err)
	n := bootstrap.Node{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: listener.Addr().String(),
	}
	// trust node
	req, err := ctrl.TrustNode(context.Background(), &n)
	require.NoError(t, err)
	err = ctrl.ConfirmTrustNode(context.Background(), &n, req)
	require.NoError(t, err)
	// connect
	err = ctrl.sender.Connect(&n, NODE.GUID())
	require.NoError(t, err)
	return NODE
}

func TestSender_Connect(t *testing.T) {
	NODE := testGenerateNodeAndTrust(t)
	defer NODE.Exit(nil)
	guid := strings.ToUpper(hex.EncodeToString(NODE.GUID()))
	err := ctrl.sender.Disconnect(guid)
	require.NoError(t, err)
}

func TestSender_Broadcast(t *testing.T) {
	NODE := testGenerateNodeAndTrust(t)
	defer func() {
		// disconnect
		guid := strings.ToUpper(hex.EncodeToString(NODE.GUID()))
		err := ctrl.sender.Disconnect(guid)
		require.NoError(t, err)
		NODE.Exit(nil)
	}()
	// ctrl.sender.Broadcast(messages.Test, messages.TestBytes)
}
