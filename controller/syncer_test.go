package controller

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/xnet"
)

func TestSyncer_Connect(t *testing.T) {
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
	err = ctrl.syncer.Connect(&node, NODE.TestGUID())
	require.NoError(t, err)
	// disconnect
	guid := base64.StdEncoding.EncodeToString(NODE.TestGUID())
	err = ctrl.syncer.Disconnect(guid)
	require.NoError(t, err)
}
