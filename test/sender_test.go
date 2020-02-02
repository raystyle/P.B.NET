package test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/messages"
	"project/internal/testsuite"

	"project/node"
)

// three common nodes connect initial node
// controller connect the initial node
// controller broadcast
func TestController_Broadcast(t *testing.T) {
	t.Skip()

	iNode := generateInitialNodeAndTrust(t)
	iNodeGUID := iNode.GUID()

	// create bootstrap
	iListener, err := iNode.GetListener(InitialNodeListenerTag)
	require.NoError(t, err)
	iAddr := iListener.Addr()
	bListener := &bootstrap.Listener{
		Mode:    iListener.Mode(),
		Network: iAddr.Network(),
		Address: iAddr.String(),
	}
	boot, key := generateBootstrap(t, bListener)
	ctrl.Test.CreateNodeRegisterRequestChannel()

	// create and run common nodes
	cNodes := make([]*node.Node, 3)
	for i := 0; i < 3; i++ {
		cNodeCfg := generateNodeConfig(t, fmt.Sprintf("Common Node %d", i))
		// must copy, because node register will cover bytes
		cNodeCfg.Register.FirstBoot = make([]byte, len(boot))
		copy(cNodeCfg.Register.FirstBoot, boot)
		cNodeCfg.Register.FirstKey = make([]byte, len(key))
		copy(cNodeCfg.Register.FirstKey, key)

		cNode, err := node.New(cNodeCfg)
		require.NoError(t, err)
		testsuite.IsDestroyed(t, cNodeCfg)

		cNode.Test.EnableTestMessage()
		cNodes[i] = cNode
		go func() {
			err := cNode.Main()
			require.NoError(t, err)
		}()
	}

	// read node register requests
	for i := 0; i < 3; i++ {
		select {
		case nrr := <-ctrl.Test.NodeRegisterRequest:
			spew.Dump(nrr)
			err = ctrl.AcceptRegisterNode(nrr, false)
			require.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Fatal("read CTRL.Test.NodeRegisterRequest timeout")
		}
	}

	// wait common nodes
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		timer := time.AfterFunc(10*time.Second, func() {
			t.Fatalf("node %d register timeout", i)
		})
		cNodes[i].Wait()
		timer.Stop()

		// connect initial node
		err := cNodes[i].Synchronize(ctx, iNodeGUID, bListener)
		require.NoError(t, err)
	}

	// broadcast
	const (
		goroutines = 256
		times      = 64
	)
	broadcast := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test broadcast %d", i))
			err := ctrl.Broadcast(messages.CMDBTest, msg)
			if err != nil {
				t.Error(err)
				return
			}
		}
	}
	for i := 0; i < goroutines; i++ {
		go broadcast(i * times)
	}

	// clean
	for i := 0; i < 3; i++ {
		cNodes[i].Exit(nil)
	}
	testsuite.IsDestroyed(t, &cNodes)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)
}

func TestNode_SendDirectly(t *testing.T) {
	Node := generateInitialNodeAndTrust(t)
	NodeGUID := Node.GUID()

	ctrl.Test.EnableRoleSendTestMessage()
	ch := ctrl.Test.CreateNodeSendTestMessageChannel(NodeGUID)

	const (
		goroutines = 256
		times      = 64
	)
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := Node.Send(messages.CMDBTest, msg)
			if err != nil {
				t.Error(err)
				return
			}
		}
	}
	for i := 0; i < goroutines; i++ {
		go send(i * times)
	}
	recv := bytes.Buffer{}
	timer := time.NewTimer(3 * time.Second)
	for i := 0; i < goroutines*times; i++ {
		timer.Reset(3 * time.Second)
		select {
		case b := <-ch:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read ctrl.Test.NodeSendTestMsg timeout i: %d", i)
		}
	}
	select {
	case <-ch:
		t.Fatal("redundancy send")
	case <-time.After(time.Second):
	}
	str := recv.String()
	for i := 0; i < goroutines*times; i++ {
		need := fmt.Sprintf("test send %d", i)
		require.True(t, strings.Contains(str, need), "lost: %s", need)
	}

	// clean
	err := ctrl.Disconnect(NodeGUID)
	require.NoError(t, err)
	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)
}
