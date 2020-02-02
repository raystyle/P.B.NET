package controller

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/messages"
	"project/internal/xnet"
)

func TestHandleNodeSendFromConnectedNode(t *testing.T) {
	const (
		address = "localhost:62300"
		times   = 10
	)
	Node := testGenerateInitialNode(t)
	NodeGUID := Node.GUID()
	defer Node.Exit(nil)
	node := &bootstrap.Listener{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: address,
	}
	// trust node
	req, err := ctrl.TrustNode(context.Background(), node)
	require.NoError(t, err)
	err = ctrl.ConfirmTrustNode(context.Background(), node, req)
	require.NoError(t, err)
	// connect
	// err = ctrl.sender.Connect(node, NodeGUID)
	// require.NoError(t, err)
	// node broadcast test message
	ch := ctrl.Test.CreateNodeSendTestMessageChannel(NodeGUID)
	msg := []byte("connected-node-send: hello controller")
	go func() {
		for i := 0; i < times; i++ {
			require.NoError(t, Node.Send(messages.CMDBTest, msg))
		}
	}()
	// read message
	for i := 0; i < times; i++ {
		select {
		case m := <-ch:
			require.Equal(t, msg, m)
		case <-time.After(time.Second):
			t.Fatal("receive broadcast message timeout")
		}
	}
	// disconnect
	guid := hex.EncodeToString(Node.GUID())
	err = ctrl.sender.Disconnect(guid)
	require.NoError(t, err)
}

func TestHandleBeaconSend(t *testing.T) {

}
