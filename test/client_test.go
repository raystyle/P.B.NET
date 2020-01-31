package test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/protocol"
	"project/internal/testsuite"

	"project/node"
)

func TestNodeClient_Synchronize(t *testing.T) {
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

	// create common node
	cNodeCfg := generateNodeConfig(t, "Common Node")
	cNodeCfg.Register.FirstBoot = boot
	cNodeCfg.Register.FirstKey = key

	ctrl.Test.CreateNodeRegisterRequestChannel()

	// run common node
	cNode, err := node.New(cNodeCfg)
	require.NoError(t, err)
	go func() {
		err := cNode.Main()
		require.NoError(t, err)
	}()

	// read Node register request
	select {
	case nrr := <-ctrl.Test.NodeRegisterRequest:
		spew.Dump(nrr)
		err = ctrl.AcceptRegisterNode(nrr, false)
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("read CTRL.Test.NodeRegisterRequest timeout")
	}

	timer := time.AfterFunc(10*time.Second, func() {
		t.Fatal("node register timeout")
	})
	cNode.Wait()
	timer.Stop()

	// try to connect initial node and start synchronize
	client, err := cNode.NewClient(context.Background(), bListener, iNodeGUID, nil)
	require.NoError(t, err)
	err = client.Connect()
	require.NoError(t, err)
	err = client.Synchronize()
	require.NoError(t, err)

	// common node send test command
	data := bytes.Buffer{}
	for i := 0; i < 1024; i++ {
		data.Write(convert.Int32ToBytes(int32(i)))
		reply, err := client.Conn.SendCommand(protocol.TestCommand, data.Bytes())
		require.NoError(t, err)
		require.Equal(t, data.Bytes(), reply)
		data.Reset()
	}

	// controller send test messages to the common node,
	// test messages will pass the initial node

	// clean
	cNode.Exit(nil)
	testsuite.IsDestroyed(t, cNode)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)
}
