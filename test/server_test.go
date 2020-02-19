package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/messages"
	"project/internal/testsuite"
	"project/internal/xnet"

	"project/beacon"
	"project/node"
)

func generateCommonNode(t *testing.T, iNode *node.Node, id int) *node.Node {
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
	cNodeCfg := generateNodeConfig(t, fmt.Sprintf("Common Node %d", id))
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

	return cNode
}

func generateBeacon(t *testing.T, node *node.Node, tag string, id int) *beacon.Beacon {
	ctrl.Test.CreateBeaconRegisterRequestChannel()

	// generate bootstrap
	listener, err := node.GetListener(tag)
	require.NoError(t, err)
	iAddr := listener.Addr()
	bListener := &bootstrap.Listener{
		Mode:    listener.Mode(),
		Network: iAddr.Network(),
		Address: iAddr.String(),
	}
	boot, key := generateBootstrap(t, bListener)

	// create Beacon and run
	beaconCfg := generateBeaconConfig(t, fmt.Sprintf("Beacon %d", id))
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

	return Beacon
}

const commonNodeListenerTag = "test_tcp"

func addNodeListener(t *testing.T, node *node.Node) *bootstrap.Listener {
	mListener := &messages.Listener{
		Tag:     commonNodeListenerTag,
		Mode:    xnet.ModeTCP,
		Network: "tcp",
		Address: "localhost:0",
	}
	err := node.AddListener(mListener)
	require.NoError(t, err)
	listener, err := node.GetListener(commonNodeListenerTag)
	require.NoError(t, err)
	return &bootstrap.Listener{
		Mode:    xnet.ModeTCP,
		Network: "tcp",
		Address: listener.Addr().String(),
	}
}

// Common Node 0 will connect the Initial Node after Common Node 1 register
//  +------------+    +----------------+    +---------------+    +---------------+
//  | Controller | -> | Initial Node 0 | <- | Common Node 0 | <- | Common Node 1 |
//  +------------+    +----------------+    +---------------+    +---------------+
func TestNodeQueryNodeKey(t *testing.T) {
	iNode, iListener, c0Node := generateInitialNodeAndCommonNode(t, 0, 0)
	iNodeGUID := iNode.GUID()
	c0NodeGUID := c0Node.GUID()

	// register Common Node 1 first, after Controller Broadcast Node key
	// the Common Node 0 connect the Initial Node
	c1Node := generateCommonNode(t, iNode, 1)
	c1NodeGUID := c1Node.GUID()

	// Common Node 0 connect the Initial Node
	err := c0Node.Synchronize(context.Background(), iNodeGUID, iListener)
	require.NoError(t, err)
	c0Listener := addNodeListener(t, c0Node)

	// Common Node 1 connect the Common Node 0
	client, err := c1Node.NewClient(context.Background(), c0Listener, c0NodeGUID)
	require.NoError(t, err)
	err = client.Connect()
	require.NoError(t, err)

	client.Close()
	testsuite.IsDestroyed(t, client)

	// clean
	c1Node.Exit(nil)
	testsuite.IsDestroyed(t, c1Node)
	c0Node.Exit(nil)
	testsuite.IsDestroyed(t, c0Node)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)

	err = ctrl.DeleteNodeUnscoped(c1NodeGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(c0NodeGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(iNodeGUID)
	require.NoError(t, err)
}

// Common Node 0 will connect the Initial Node after Beacon 0 register
//  +------------+    +----------------+    +---------------+    +----------+
//  | Controller | -> | Initial Node 0 | <- | Common Node 0 | <- | Beacon 0 |
//  +------------+    +----------------+    +---------------+    +----------+
func TestNodeQueryBeaconKey(t *testing.T) {
	iNode, iListener, cNode := generateInitialNodeAndCommonNode(t, 0, 0)
	iNodeGUID := iNode.GUID()
	cNodeGUID := cNode.GUID()

	// register Beacon first, after Controller Broadcast Beacon key
	// the Common Node 0 connect the Initial Node
	Beacon := generateBeacon(t, iNode, initialNodeListenerTag, 0)
	beaconGUID := Beacon.GUID()

	// Common Node 0 connect the Initial Node
	err := cNode.Synchronize(context.Background(), iNodeGUID, iListener)
	require.NoError(t, err)
	cListener := addNodeListener(t, cNode)

	// Beacon connect the Common Node
	client, err := Beacon.NewClient(context.Background(), cListener, cNodeGUID, nil)
	require.NoError(t, err)
	err = client.Connect()
	require.NoError(t, err)

	client.Close()
	testsuite.IsDestroyed(t, client)

	// clean
	Beacon.Exit(nil)
	testsuite.IsDestroyed(t, Beacon)
	cNode.Exit(nil)
	testsuite.IsDestroyed(t, cNode)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)

	err = ctrl.DeleteBeaconUnscoped(beaconGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(cNodeGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(iNodeGUID)
	require.NoError(t, err)
}
