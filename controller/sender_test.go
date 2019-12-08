package controller

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/messages"
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
		guid := strings.ToUpper(hex.EncodeToString(NODE.GUID()))
		err := ctrl.sender.Disconnect(guid)
		require.NoError(t, err)
		NODE.Exit(nil)
	}()
	for i := 0; i < 1024; i++ {
		msg := []byte(fmt.Sprintf("test broadcast %d", i))
		require.NoError(t, ctrl.sender.Broadcast(messages.CMDBytesTest, msg))
	}
	recv := bytes.Buffer{}
	for i := 0; i < 1024; i++ {
		select {
		case b := <-NODE.Debug.Broadcast:
			recv.Write(b)
			recv.WriteString("\n")
		case <-time.After(time.Second):
			t.Fatal("read NODE.Debug.Broadcast timeout")
		}
	}
	select {
	case <-NODE.Debug.Broadcast:
		t.Fatal("redundancy broadcast")
	case <-time.After(time.Second):
	}
	str := recv.String()
	for i := 0; i < 1024; i++ {
		need := fmt.Sprintf("test broadcast %d", i)
		require.True(t, strings.Contains(str, need), "lost: %s", need)
	}
}
