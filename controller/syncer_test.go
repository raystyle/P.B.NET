package controller

import (
	"encoding/base64"
	"testing"
	"time"

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
	err = ctrl.syncer.Connect(&node, NODE.TestGetGUID())
	require.NoError(t, err)
	// disconnect
	guid := base64.StdEncoding.EncodeToString(NODE.TestGetGUID())
	err = ctrl.syncer.Disconnect(guid)
	require.NoError(t, err)
}

func TestNodeBroadcastFromConnectedNodes(t *testing.T) {
	const (
		address1 = "localhost:62300"
		times    = 10
	)
	testInitCtrl(t)
	NODE1 := testGenerateNode(t, true)
	defer NODE1.Exit(nil)
	node1 := bootstrap.Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: address1,
	}
	// trust node
	req, err := ctrl.TrustNode(&node1)
	require.NoError(t, err)
	err = ctrl.ConfirmTrustNode(&node1, req)
	require.NoError(t, err)
	// connect
	err = ctrl.syncer.Connect(&node1, NODE1.TestGetGUID())
	require.NoError(t, err)
	// node broadcast test message
	msg := []byte("node-broadcast: hello controller")
	ctrl.Debug.NodeBroadcastChan = make(chan []byte, times)
	go func() {
		for i := 0; i < times; i++ {
			result := NODE1.TestBroadcast(msg)
			require.NoError(t, result.Err)
			require.Equal(t, 1, result.Success)
		}
	}()
	// read
	for i := 0; i < times; i++ {
		select {
		case m := <-ctrl.Debug.NodeBroadcastChan:
			require.Equal(t, msg, m)
		case <-time.After(time.Second):
			t.Fatal("receive broadcast message timeout")
		}
	}
	// disconnect
	guid := base64.StdEncoding.EncodeToString(NODE1.TestGetGUID())
	err = ctrl.syncer.Disconnect(guid)
	require.NoError(t, err)
}

func TestNodeSyncSendFromConnectedNode(t *testing.T) {
	const (
		address = "localhost:62300"
		times   = 1
	)
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
	msg := []byte("node-send: hello controller")
	ctrl.Debug.NodeSyncSendChan = make(chan []byte, times)
	go func() {
		for i := 0; i < times; i++ {
			result := NODE.TestSend(msg)
			require.NoError(t, result.Err)
			require.Equal(t, 1, result.Success)
		}
	}()
	// read
	for i := 0; i < times; i++ {
		select {
		case m := <-ctrl.Debug.NodeSyncSendChan:
			require.Equal(t, msg, m)
		case <-time.After(time.Second):
			t.Fatal("receive broadcast message timeout")
		}
	}
	// disconnect
	guid := base64.StdEncoding.EncodeToString(NODE.TestGetGUID())
	err = ctrl.syncer.Disconnect(guid)
	require.NoError(t, err)
	// wait db cache sync
	ctrl.TestSyncDBCache()
	// check node send and controller receive

}
