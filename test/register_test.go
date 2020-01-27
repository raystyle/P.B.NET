package test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"

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
	cNodeCfg := generateNodeConfig(t)
	cNodeCfg.Register.FirstBoot = boot
	cNodeCfg.Register.FirstKey = key

	// run common node
	cNode, err := node.New(cNodeCfg)
	require.NoError(t, err)
	err = cNode.Main()
	// require.NoError(t, err)

	// clean
	iNode.Exit(nil)
	// testsuite.IsDestroyed(t, iNode)
	cNode.Exit(nil)
	// testsuite.IsDestroyed(t, cNode)
}
