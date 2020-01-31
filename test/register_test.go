package test

import (
	"context"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/testsuite"

	"project/node"
)

func TestNodeRegister(t *testing.T) {
	iNode := generateInitialNodeAndTrust(t)

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

	// try to connect initial node
	client, err := cNode.NewClient(context.Background(), bListener, iNode.GUID(), nil)
	require.NoError(t, err)
	err = client.Connect()
	require.NoError(t, err)

	// clean
	cNode.Exit(nil)
	testsuite.IsDestroyed(t, cNode)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)
}
