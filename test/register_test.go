package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/testsuite"

	"project/beacon"
	"project/node"
)

// generateInitialNodeAndCommonNode is used to generate an Initial Node
// and trust it, then generate a Common Node and register it.
func generateInitialNodeAndCommonNode(t testing.TB, iID, cID int) (
	*node.Node, *bootstrap.Listener, *node.Node) {
	iNode := generateInitialNodeAndTrust(t, iID)

	ctrl.Test.CreateNodeRegisterRequestChannel()

	// generate bootstrap
	listener, err := iNode.GetListener(initialNodeListenerTag)
	require.NoError(t, err)
	iAddr := listener.Addr()
	iListener := &bootstrap.Listener{
		Mode:    listener.Mode(),
		Network: iAddr.Network(),
		Address: iAddr.String(),
	}
	boot, key := generateBootstrap(t, iListener)

	// create Common Node and run
	cNodeCfg := generateNodeConfig(t, fmt.Sprintf("Common Node %d", cID))
	cNodeCfg.Register.FirstBoot = boot
	cNodeCfg.Register.FirstKey = key
	cNode, err := node.New(cNodeCfg)
	require.NoError(t, err)
	testsuite.IsDestroyed(t, cNodeCfg)
	go func() {
		err := cNode.Main()
		require.NoError(t, err)
	}()

	// read Node register request
	select {
	case nrr := <-ctrl.Test.NodeRegisterRequest:
		// spew.Dump(nrr)
		err = ctrl.AcceptRegisterNode(nrr, nil, false)
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("read Ctrl.Test.NodeRegisterRequest timeout")
	}

	// wait Common Node
	timer := time.AfterFunc(10*time.Second, func() {
		t.Fatal("node register timeout")
	})
	cNode.Wait()
	timer.Stop()

	return iNode, iListener, cNode
}

func TestNodeRegister(t *testing.T) {
	iNode, iListener, cNode := generateInitialNodeAndCommonNode(t, 0, 0)
	iNodeGUID := iNode.GUID()
	cNodeGUID := cNode.GUID()

	client, err := cNode.NewClient(context.Background(), iListener, iNodeGUID)
	require.NoError(t, err)
	err = client.Connect()
	require.NoError(t, err)

	// clean
	cNode.Exit(nil)
	testsuite.IsDestroyed(t, cNode)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)

	err = ctrl.DeleteNodeUnscoped(cNodeGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(iNodeGUID)
	require.NoError(t, err)
}

// generateInitialNodeAndBeacon is used to generate an Initial Node
// and trust it, then generate a Beacon and register it.
func generateInitialNodeAndBeacon(t testing.TB, iID, bID int) (
	*node.Node, *bootstrap.Listener, *beacon.Beacon) {
	iNode := generateInitialNodeAndTrust(t, iID)
	ctrl.Test.CreateBeaconRegisterRequestChannel()

	// generate bootstrap
	listener, err := iNode.GetListener(initialNodeListenerTag)
	require.NoError(t, err)
	iAddr := listener.Addr()
	iListener := &bootstrap.Listener{
		Mode:    listener.Mode(),
		Network: iAddr.Network(),
		Address: iAddr.String(),
	}
	boot, key := generateBootstrap(t, iListener)

	// create Beacon and run
	beaconCfg := generateBeaconConfig(t, fmt.Sprintf("Beacon %d", bID))
	beaconCfg.Register.FirstBoot = boot
	beaconCfg.Register.FirstKey = key
	Beacon, err := beacon.New(beaconCfg)
	require.NoError(t, err)
	go func() {
		err := Beacon.Main()
		require.NoError(t, err)
	}()

	// read Beacon register request
	select {
	case brr := <-ctrl.Test.BeaconRegisterRequest:
		// spew.Dump(brr)
		err = ctrl.AcceptRegisterBeacon(brr, nil)
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("read Ctrl.Test.BeaconRegisterRequest timeout")
	}
	timer := time.AfterFunc(10*time.Second, func() {
		t.Fatal("beacon register timeout")
	})
	Beacon.Wait()
	timer.Stop()

	return iNode, iListener, Beacon
}

func TestBeaconRegister(t *testing.T) {
	iNode, iListener, Beacon := generateInitialNodeAndBeacon(t, 0, 0)
	iNodeGUID := iNode.GUID()
	beaconGUID := Beacon.GUID()

	// try to connect Initial Node
	client, err := Beacon.NewClient(context.Background(), iListener, iNodeGUID, nil)
	require.NoError(t, err)
	err = client.Connect()
	require.NoError(t, err)

	// clean
	Beacon.Exit(nil)
	testsuite.IsDestroyed(t, Beacon)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)

	err = ctrl.DeleteBeaconUnscoped(beaconGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(iNodeGUID)
	require.NoError(t, err)
}
