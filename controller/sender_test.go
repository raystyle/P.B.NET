package controller

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/xnet"
)

func TestSender_Broadcast(t *testing.T) {
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
	err = ctrl.syncer.Connect(&node, NODE.TestGetGUID())
	require.NoError(t, err)
	// node broadcast test message
	msg := []byte("ctrl-broadcast: hello node")
	// TODO node syncer
	ctrl.Debug.NodeBroadcastChan = make(chan []byte, 1)
	result := NODE.TestBroadcast(msg)
	require.NoError(t, result.Err)
	require.Equal(t, 1, result.Success)
	select {
	case m := <-ctrl.Debug.NodeBroadcastChan:
		require.Equal(t, msg, m)
	case <-time.After(time.Second):
		t.Fatal("receive broadcast message timeout")
	}
	// disconnect
	guid := base64.StdEncoding.EncodeToString(NODE.TestGetGUID())
	err = ctrl.syncer.Disconnect(guid)
	require.NoError(t, err)
}
