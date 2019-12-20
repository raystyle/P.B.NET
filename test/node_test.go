package test

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

func testGenerateNodeAndConnect(t testing.TB) *node.Node {
	NODE := generateNodeWithListener(t)
	listener, err := NODE.GetListener(initNodeListenerTag)
	require.NoError(t, err)
	n := bootstrap.Node{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: listener.Addr().String(),
	}
	// controller trust node
	req, err := ctrl.TrustNode(context.Background(), &n)
	require.NoError(t, err)
	err = ctrl.ConfirmTrustNode(context.Background(), &n, req)
	require.NoError(t, err)
	// controller connect node
	err = ctrl.Connect(&n, NODE.GUID())
	require.NoError(t, err)
	return NODE
}

func TestNode_SendDirectly(t *testing.T) {
	NODE := testGenerateNodeAndConnect(t)
	const (
		goRoutines = 256
		times      = 64
	)
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := NODE.Send(messages.CMDBTest, msg)
			if err != nil {
				t.Error(err)
				return
			}
		}
	}
	for i := 0; i < goRoutines; i++ {
		go send(i * times)
	}
	recv := bytes.Buffer{}
	timer := time.NewTimer(3 * time.Second)
	for i := 0; i < goRoutines*times; i++ {
		timer.Reset(3 * time.Second)
		select {
		case b := <-ctrl.Debug.NodeSend:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read ctrl.Debug.NodeSend timeout i: %d", i)
		}
	}
	select {
	case <-NODE.Debug.Send:
		t.Fatal("redundancy send")
	case <-time.After(time.Second):
	}
	str := recv.String()
	for i := 0; i < goRoutines*times; i++ {
		need := fmt.Sprintf("test send %d", i)
		require.True(t, strings.Contains(str, need), "lost: %s", need)
	}
	// clean
	guid := strings.ToUpper(hex.EncodeToString(NODE.GUID()))
	err := ctrl.Disconnect(guid)
	require.NoError(t, err)
	NODE.Exit(nil)

	// testsuite.IsDestroyed(t, NODE)
}
