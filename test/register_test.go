package test

import (
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
	bListener := bootstrap.Listener{
		Mode:    iListener.Mode(),
		Network: iAddr.Network(),
		Address: iAddr.String(),
	}
	boot, key := generateBootstrap(t, &bListener)

	// create common node
	cNodeCfg := generateNodeConfig(t, "Common Node")
	cNodeCfg.Register.FirstBoot = boot
	cNodeCfg.Register.FirstKey = key

	ctrl.Test.CreateNodeRegisterRequestChannel()

	// run common node
	cNode, err := node.New(cNodeCfg)
	require.NoError(t, err)
	go func() {
		err = cNode.Main()
		require.Error(t, err)
	}()

	// read Node register request
	select {
	case req := <-ctrl.Test.NodeRegisterRequest:
		spew.Dump(req)
	case <-time.After(3 * time.Second):
		t.Fatal("read CTRL.Test.NodeRegisterRequestChannel timeout")
	}

	// clean
	cNode.Exit(nil)
	testsuite.IsDestroyed(t, cNode)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)
}
