package controller

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/messages"
	"project/internal/protocol"
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

func TestSender_Send(t *testing.T) {
	NODE := testGenerateNodeAndTrust(t)
	defer func() {
		guid := strings.ToUpper(hex.EncodeToString(NODE.GUID()))
		err := ctrl.sender.Disconnect(guid)
		require.NoError(t, err)
		NODE.Exit(nil)
	}()
	// send to Node
	roleGUID := NODE.GUID()
	var (
		goRoutines = 2
		number     = 202400
	)
	send := func(start int) {
		for i := start; i < start+number; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := ctrl.sender.Send(protocol.Node, roleGUID, messages.CMDBytesTest, msg)
			require.NoError(t, err)
		}
	}
	for i := 0; i < goRoutines; i++ {
		go send(i * number)
	}
	// recv := bytes.Buffer{} // TODO memory

	go func() {
		for {
			debug.FreeOSMemory()
			time.Sleep(5 * time.Second)
		}
	}()

	timer := time.NewTimer(5 * time.Second)
	for i := 0; i < goRoutines*number; i++ {
		timer.Reset(5 * time.Second)
		select {
		case <-NODE.Debug.Send:
		// case b := <-NODE.Debug.Send:
		// recv.Write(b)
		// recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read NODE.Debug.Send timeout id: %d", i)
		}
	}
	select {
	case <-NODE.Debug.Send:
		t.Fatal("redundancy send")
	case <-time.After(time.Second):
	}
	// str := recv.String()
	for i := 0; i < goRoutines*number; i++ {
		// need := fmt.Sprintf("test send %d", i)
		// require.True(t, strings.Contains(str, need), "lost: %s", need)
	}
}
