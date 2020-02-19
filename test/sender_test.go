package test

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/guid"
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

func getNodeListener(t *testing.T, node *node.Node, tag string) *bootstrap.Listener {
	listener, err := node.GetListener(tag)
	require.NoError(t, err)
	return &bootstrap.Listener{
		Mode:    xnet.ModeTCP,
		Network: "tcp",
		Address: listener.Addr().String(),
	}
}

// 3 * (A Common Node Connect the Initial Node)
//
//  +------------+    +----------------+    +---------------+
//  |            | -> | Initial Node 0 | <- | Common Node 0 |
//  |            |    +----------------+    +---------------+
//  |            |
//  |            |    +----------------+    +---------------+
//  | Controller | -> | Initial Node 1 | <- | Common Node 1 |
//  |            |    +----------------+    +---------------+
//  |            |
//  |            |    +----------------+    +---------------+
//  |            | -> | Initial Node 2 | <- | Common Node 2 |
//  +------------+    +----------------+    +---------------+
//
func buildNodeNetworkWithCI(t *testing.T, recvMsg bool) ([]*node.Node, []*node.Node) {
	const num = 3
	var (
		iNodes [num]*node.Node
		cNodes [num]*node.Node
	)
	for i := 0; i < num; i++ {
		iNode, iListener, cNode := generateInitialNodeAndCommonNode(t, i, i)

		if recvMsg {
			iNode.Test.EnableTestMessage()
			cNode.Test.EnableTestMessage()
		}

		// try to connect Initial Node and start to synchronize
		err := cNode.Synchronize(context.Background(), iNode.GUID(), iListener)
		require.NoError(t, err)

		iNodes[i] = iNode
		cNodes[i] = cNode
	}
	return iNodes[:], cNodes[:]
}

// 3 * (Initial Node connect the Common Node Connect)
//
//  +------------+    +----------------+
//  |            | -> | Initial Node 0 |
//  |            |    +----------------+
//  |            |            ↓
//  |            |    +---------------+
//  |            | -> | Common Node 0 |
//  |            |    +---------------+
//  |            |
//  |            |    +----------------+
//  |            | -> | Initial Node 1 |
//  |            |    +----------------+
//  | Controller |            ↓
//  |            |    +---------------+
//  |            | -> | Common Node 1 |
//  |            |    +---------------+
//  |            |
//  |            |    +----------------+
//  |            | -> | Initial Node 2 |
//  |            |    +----------------+
//  |            |            ↓
//  |            |    +---------------+
//  |            | -> | Common Node 2 |
//  +------------+    +---------------+
//
func buildNodeNetworkWithIC(t *testing.T, recvMsg bool) ([]*node.Node, []*node.Node) {
	const num = 3
	var (
		iNodes [num]*node.Node
		cNodes [num]*node.Node
	)
	for i := 0; i < num; i++ {
		iNode, _, cNode := generateInitialNodeAndCommonNode(t, i, i)

		if recvMsg {
			iNode.Test.EnableTestMessage()
			cNode.Test.EnableTestMessage()
		}

		cListener := addNodeListener(t, cNode)

		ctx := context.Background()

		// Controller must connect the Common Node, otherwise the Common Node
		// can't query Node key from Controller
		err := ctrl.Synchronize(ctx, cNode.GUID(), cListener)
		require.NoError(t, err)

		// Initial Node connect the Common Node and start to synchronize
		err = iNode.Synchronize(ctx, cNode.GUID(), cListener)
		require.NoError(t, err)

		iNodes[i] = iNode
		cNodes[i] = cNode
	}
	return iNodes[:], cNodes[:]
}

// mix network environment
//
//  +------------+    +---------------+    +---------------+
//  |            | -> | Initial Node  | <- | Common Node 1 |
//  |            |    +---------------+    +---------------+
//  | Controller |            ↓         ↖         ↑
//  |            |    +---------------+    +---------------+
//  |            | -> | Common Node 0 | -> | Common Node 2 |
//  +------------+    +---------------+    +---------------+
//
func buildNodeNetworkWithMix(t *testing.T, recvMsg bool) ([]*node.Node, []*node.Node) {
	iNode := generateInitialNodeAndTrust(t, 0)
	if recvMsg {
		iNode.Test.EnableTestMessage()
	}
	iNodeGUID := iNode.GUID()

	// create Common Nodes
	const num = 3
	cNodes := make([]*node.Node, num)
	for i := 0; i < num; i++ {
		cNodes[i] = generateCommonNode(t, iNode, i)
		if recvMsg {
			cNodes[i].Test.EnableTestMessage()
		}
	}

	ctx := context.Background()

	// Controller and Initial Node connect Common Node 0
	c0Listener := addNodeListener(t, cNodes[0])
	c0GUID := cNodes[0].GUID()
	err := ctrl.Synchronize(ctx, c0GUID, c0Listener)
	require.NoError(t, err)
	err = iNode.Synchronize(ctx, c0GUID, c0Listener)
	require.NoError(t, err)

	// Common Node 1 connect the Initial Node
	iListener := getNodeListener(t, iNode, initialNodeListenerTag)
	err = cNodes[1].Synchronize(ctx, iNodeGUID, iListener)
	require.NoError(t, err)

	// Common Node 2 Connect the Common Node 1 and the Initial Node
	c1Listener := addNodeListener(t, cNodes[1])
	c1GUID := cNodes[1].GUID()
	err = cNodes[2].Synchronize(ctx, c1GUID, c1Listener)
	require.NoError(t, err)
	err = cNodes[2].Synchronize(ctx, iNodeGUID, iListener)
	require.NoError(t, err)

	// Common Node 0 connect the Common Node 2
	c2Listener := addNodeListener(t, cNodes[2])
	c2GUID := cNodes[2].GUID()
	err = cNodes[0].Synchronize(ctx, c2GUID, c2Listener)
	require.NoError(t, err)

	return []*node.Node{iNode}, cNodes
}

// 3 * (A Common Node Connect the Initial Node, a Beacon connect the Common Node)
//
//  +------------+    +----------------+    +---------------+    +----------+
//  |            | -> | Initial Node 0 | <- | Common Node 0 | <- | Beacon 0 |
//  |            |    +----------------+    +---------------+    +----------+
//  |            |
//  |            |    +----------------+    +---------------+    +----------+
//  | Controller | -> | Initial Node 1 | <- | Common Node 1 | <- | Beacon 1 |
//  |            |    +----------------+    +---------------+    +----------+
//  |            |
//  |            |    +----------------+    +---------------+    +----------+
//  |            | -> | Initial Node 2 | <- | Common Node 2 | <- | Beacon 2 |
//  +------------+    +----------------+    +---------------+    +----------+
//
func buildBeaconNetworkWithCI(t *testing.T, recvMsg bool) ([]*node.Node, []*beacon.Beacon) {
	const num = 3
	var (
		nodes   [2 * num]*node.Node
		beacons [num]*beacon.Beacon
	)
	for i := 0; i < num; i++ {
		iNode, iListener, cNode := generateInitialNodeAndCommonNode(t, i, i)

		ctx := context.Background()

		// try to connect Initial Node and start to synchronize
		err := cNode.Synchronize(ctx, iNode.GUID(), iListener)
		require.NoError(t, err)

		// add listener to Common Node
		listener := addNodeListener(t, cNode)

		// create Beacon
		Beacon := generateBeacon(t, cNode, commonNodeListenerTag, i)
		err = Beacon.Synchronize(ctx, cNode.GUID(), listener)
		require.NoError(t, err)
		if recvMsg {
			Beacon.Test.EnableTestMessage()
		}

		ctrl.EnableInteractiveMode(Beacon.GUID())

		nodes[2*i] = iNode
		nodes[2*i+1] = cNode
		beacons[i] = Beacon
	}
	return nodes[:], beacons[:]
}

// 3 * (Initial Node connect the Common Node Connect, a Beacon connect
// the Initial Node and the Common Node)
//
//  +------------+    +----------------+    +----------+
//  |            | -> | Initial Node 0 | <- |          |
//  |            |    +----------------+    |          |
//  |            |            ↓             | Beacon 0 |
//  |            |    +---------------+     |          |
//  |            | -> | Common Node 0 |  <- |          |
//  |            |    +---------------+     +----------+
//  |            |
//  |            |    +----------------+    +----------+
//  |            | -> | Initial Node 1 | <- |          |
//  |            |    +----------------+    |          |
//  | Controller |            ↓             | Beacon 1 |
//  |            |    +---------------+     |          |
//  |            | -> | Common Node 1 |  <- |          |
//  |            |    +---------------+     +----------+
//  |            |
//  |            |    +----------------+    +----------+
//  |            | -> | Initial Node 2 | <- |          |
//  |            |    +----------------+    |          |
//  |            |            ↓             | Beacon 2 |
//  |            |    +---------------+     |          |
//  |            | -> | Common Node 2 |  <- |          |
//  +------------+    +---------------+     +----------+
//
func buildBeaconNetworkWithIC(t *testing.T, recvMsg bool) ([]*node.Node, []*beacon.Beacon) {
	const num = 3
	var (
		nodes   [2 * num]*node.Node
		beacons [num]*beacon.Beacon
	)
	for i := 0; i < num; i++ {
		iNode, iListener, cNode := generateInitialNodeAndCommonNode(t, i, i)

		cListener := addNodeListener(t, cNode)
		ctx := context.Background()

		// Controller must connect the Common Node, otherwise the Common Node
		// can't query Node key from Controller
		err := ctrl.Synchronize(ctx, cNode.GUID(), cListener)
		require.NoError(t, err)

		// Initial Node connect the Common Node and start to synchronize
		err = iNode.Synchronize(ctx, cNode.GUID(), cListener)
		require.NoError(t, err)

		// create Beacon
		Beacon := generateBeacon(t, cNode, commonNodeListenerTag, i)
		err = Beacon.Synchronize(ctx, cNode.GUID(), cListener)
		require.NoError(t, err)
		err = Beacon.Synchronize(ctx, iNode.GUID(), iListener)
		require.NoError(t, err)
		if recvMsg {
			Beacon.Test.EnableTestMessage()
		}
		ctrl.EnableInteractiveMode(Beacon.GUID())

		nodes[2*i] = iNode
		nodes[2*i+1] = cNode
		beacons[i] = Beacon
	}
	return nodes[:], beacons[:]
}

// mix network environment
//
//                               +--------------+
//                               |   Beacon 0   |
//                               +--------------+
//                                 ↓          ↓
//  +------------+    +---------------+    +---------------+    +----------+
//  |            | -> | Initial Node  | <- | Common Node 1 | <- |          |
//  |            |    +---------------+    +---------------+    |          |
//  | Controller |            ↓         ↖         ↑            | Beacon 2 |
//  |            |    +---------------+    +---------------+    |          |
//  |            | -> | Common Node 0 | -> | Common Node 2 | <- |          |
//  +------------+    +---------------+    +---------------+    +----------+
//                                 ↑          ↑
//                               +--------------+
//                               |   Beacon 1   |
//                               +--------------+
//
func buildBeaconNetworkWithMix(t *testing.T, recvMsg bool) ([]*node.Node, []*beacon.Beacon) {
	iNode := generateInitialNodeAndTrust(t, 0)
	iNodeGUID := iNode.GUID()

	// create Common Nodes
	const num = 3
	cNodes := make([]*node.Node, num)
	for i := 0; i < num; i++ {
		cNodes[i] = generateCommonNode(t, iNode, i)
	}

	ctx := context.Background()

	// Controller and Initial Node connect Common Node 0
	c0Listener := addNodeListener(t, cNodes[0])
	c0GUID := cNodes[0].GUID()
	err := ctrl.Synchronize(ctx, c0GUID, c0Listener)
	require.NoError(t, err)
	err = iNode.Synchronize(ctx, c0GUID, c0Listener)
	require.NoError(t, err)

	// Common Node 1 connect the Initial Node
	iListener := getNodeListener(t, iNode, initialNodeListenerTag)
	err = cNodes[1].Synchronize(ctx, iNodeGUID, iListener)
	require.NoError(t, err)

	// Common Node 2 Connect the Common Node 1 and the Initial Node
	c1Listener := addNodeListener(t, cNodes[1])
	c1GUID := cNodes[1].GUID()
	err = cNodes[2].Synchronize(ctx, c1GUID, c1Listener)
	require.NoError(t, err)
	err = cNodes[2].Synchronize(ctx, iNodeGUID, iListener)
	require.NoError(t, err)

	// Common Node 0 connect the Common Node 2
	c2Listener := addNodeListener(t, cNodes[2])
	c2GUID := cNodes[2].GUID()
	err = cNodes[0].Synchronize(ctx, c2GUID, c2Listener)
	require.NoError(t, err)

	// create Beacons
	beacons := make([]*beacon.Beacon, num)

	// Beacon 0 connect the Initial Node and the Common Node 1
	beacons[0] = generateBeacon(t, iNode, initialNodeListenerTag, 0)
	err = beacons[0].Synchronize(ctx, iNode.GUID(), iListener)
	require.NoError(t, err)
	err = beacons[0].Synchronize(ctx, c1GUID, c1Listener)
	require.NoError(t, err)
	if recvMsg {
		beacons[0].Test.EnableTestMessage()
	}
	ctrl.EnableInteractiveMode(beacons[0].GUID())

	// Beacon 1 connect the Common Node 0 and the Common Node 2
	beacons[1] = generateBeacon(t, cNodes[0], commonNodeListenerTag, 1)
	err = beacons[1].Synchronize(ctx, c0GUID, c0Listener)
	require.NoError(t, err)
	err = beacons[1].Synchronize(ctx, c2GUID, c2Listener)
	require.NoError(t, err)
	if recvMsg {
		beacons[1].Test.EnableTestMessage()
	}
	ctrl.EnableInteractiveMode(beacons[1].GUID())

	// Beacon 2 connect the Common Node 1 and the Common Node 2
	beacons[2] = generateBeacon(t, cNodes[1], commonNodeListenerTag, 2)
	err = beacons[2].Synchronize(ctx, c1GUID, c1Listener)
	require.NoError(t, err)
	err = beacons[2].Synchronize(ctx, c2GUID, c2Listener)
	require.NoError(t, err)
	if recvMsg {
		beacons[2].Test.EnableTestMessage()
	}
	ctrl.EnableInteractiveMode(beacons[2].GUID())
	return append([]*node.Node{iNode}, cNodes[:]...), beacons[:]
}

func TestCtrl_Broadcast_CI(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithCI(t, true)
	testCtrlBroadcast(t, iNodes, cNodes)
}

func TestCtrl_Broadcast_IC(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithIC(t, true)
	testCtrlBroadcast(t, iNodes[:], cNodes[:])
}

func TestCtrl_Broadcast_Mix(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithMix(t, true)
	testCtrlBroadcast(t, iNodes, cNodes)
}

// Each Node will receive the message that Controller broadcast.
func testCtrlBroadcast(t *testing.T, iNodes, cNodes []*node.Node) {
	const (
		goroutines = 64
		times      = 64
	)
	broadcast := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test broadcast with deflate %d", i))
			err := ctrl.Broadcast(messages.CMDBTest, msg, true)
			require.NoError(t, err)
			msg = []byte(fmt.Sprintf("test broadcast without deflate %d", i))
			err = ctrl.Broadcast(messages.CMDBTest, msg, false)
			require.NoError(t, err)
		}
	}

	wg := sync.WaitGroup{}
	read := func(n int, node *node.Node, initial bool) {
		defer wg.Done()
		var prefix string
		if initial {
			prefix = "Initial Node[%d]"
		} else {
			prefix = "Common Node[%d]"
		}
		recv := make(map[string]struct{}, 2*goroutines*times)
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		for i := 0; i < 2*goroutines*times; i++ {
			timer.Reset(3 * time.Second)
			select {
			case msg := <-node.Test.BroadcastTestMsg:
				recv[string(msg)] = struct{}{}
			case <-timer.C:
				format := "read " + prefix + ".Test.BroadcastTestMsg timeout i: %d"
				t.Fatalf(format, n, i)
			}
		}
		select {
		case <-node.Test.BroadcastTestMsg:
			t.Fatalf(prefix+" read redundancy broadcast", n)
		case <-time.After(time.Second):
		}
		for i := 0; i < goroutines*times; i++ {
			format := prefix + "lost: %s"
			withDeflate := fmt.Sprintf("test broadcast with deflate %d", i)
			_, ok := recv[withDeflate]
			require.Truef(t, ok, format, n, withDeflate)
			withoutDeflate := fmt.Sprintf("test broadcast without deflate %d", i)
			_, ok = recv[withoutDeflate]
			require.Truef(t, ok, format, n, withoutDeflate)
		}
	}

	// broadcast and read message
	for i := 0; i < goroutines; i++ {
		go broadcast(i * times)
	}
	for i := 0; i < len(iNodes); i++ {
		wg.Add(1)
		go read(i, iNodes[i], true)
	}
	for i := 0; i < len(cNodes); i++ {
		wg.Add(1)
		go read(i, cNodes[i], false)
	}
	wg.Wait()

	// clean
	for i := 0; i < len(cNodes); i++ {
		cNode := cNodes[i]
		cNodes[i] = nil
		cNodeGUID := cNode.GUID()

		cNode.Exit(nil)
		testsuite.IsDestroyed(t, cNode)

		err := ctrl.DeleteNodeUnscoped(cNodeGUID)
		require.NoError(t, err)
	}
	for i := 0; i < len(iNodes); i++ {
		iNode := iNodes[i]
		iNodes[i] = nil
		iNodeGUID := iNode.GUID()

		iNode.Exit(nil)
		testsuite.IsDestroyed(t, iNode)

		err := ctrl.DeleteNodeUnscoped(iNodeGUID)
		require.NoError(t, err)
	}
}

func TestCtrl_SendToNode_CI(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithCI(t, true)
	testCtrlSendToNode(t, iNodes, cNodes)
}

func TestCtrl_SendToNode_IC(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithIC(t, true)
	testCtrlSendToNode(t, iNodes, cNodes)
}

func TestCtrl_SendToNode_Mix(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithMix(t, true)
	testCtrlSendToNode(t, iNodes, cNodes)
}

// Controller will send message to each Node.
func testCtrlSendToNode(t *testing.T, iNodes, cNodes []*node.Node) {
	const (
		goroutines = 128
		times      = 64
	)
	ctx := context.Background()
	send := func(start int, guid *guid.GUID) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send with deflate %d", i))
			err := ctrl.SendToNode(ctx, guid, messages.CMDBTest, msg, true)
			require.NoError(t, err)
			msg = []byte(fmt.Sprintf("test send without deflate %d", i))
			err = ctrl.SendToNode(ctx, guid, messages.CMDBTest, msg, false)
			require.NoError(t, err)
		}
	}

	wg := sync.WaitGroup{}
	sendAndRead := func(n int, node *node.Node, initial bool) {
		defer wg.Done()
		var prefix string
		if initial {
			prefix = "Initial Node[%d]"
		} else {
			prefix = "Common Node[%d]"
		}
		// send
		for i := 0; i < goroutines; i++ {
			go send(i*times, node.GUID())
		}
		// read
		recv := make(map[string]struct{}, 2*goroutines*times)
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		for i := 0; i < 2*goroutines*times; i++ {
			timer.Reset(3 * time.Second)
			select {
			case msg := <-node.Test.SendTestMsg:
				recv[string(msg)] = struct{}{}
			case <-timer.C:
				format := "read " + prefix + ".Test.SendTestMsg timeout i: %d"
				t.Fatalf(format, n, i)
			}
		}
		select {
		case <-node.Test.SendTestMsg:
			t.Fatalf(prefix+" read redundancy send", n)
		case <-time.After(time.Second):
		}
		for i := 0; i < goroutines*times; i++ {
			format := prefix + "lost %s"
			withDeflate := fmt.Sprintf("test send with deflate %d", i)
			_, ok := recv[withDeflate]
			require.Truef(t, ok, format, n, withDeflate)
			withoutDeflate := fmt.Sprintf("test send without deflate %d", i)
			_, ok = recv[withoutDeflate]
			require.Truef(t, ok, format, n, withoutDeflate)
		}
	}
	// send and read
	for i := 0; i < len(iNodes); i++ {
		wg.Add(1)
		go sendAndRead(i, iNodes[i], true)
	}
	for i := 0; i < len(cNodes); i++ {
		wg.Add(1)
		go sendAndRead(i, cNodes[i], false)
	}
	wg.Wait()

	// clean
	for i := 0; i < len(cNodes); i++ {
		cNode := cNodes[i]
		cNodes[i] = nil
		cNodeGUID := cNode.GUID()

		cNode.Exit(nil)
		testsuite.IsDestroyed(t, cNode)

		err := ctrl.DeleteNodeUnscoped(cNodeGUID)
		require.NoError(t, err)
	}
	for i := 0; i < len(iNodes); i++ {
		iNode := iNodes[i]
		iNodes[i] = nil
		iNodeGUID := iNode.GUID()

		iNode.Exit(nil)
		testsuite.IsDestroyed(t, iNode)

		err := ctrl.DeleteNodeUnscoped(iNodeGUID)
		require.NoError(t, err)
	}
}

func TestCtrl_SendToBeacon_CI(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithCI(t, true)
	testCtrlSendToBeacon(t, nodes, beacons)
}

func TestCtrl_SendToBeacon_IC(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithIC(t, true)
	testCtrlSendToBeacon(t, nodes, beacons)
}

func TestCtrl_SendToBeacon_Mix(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithMix(t, true)
	testCtrlSendToBeacon(t, nodes, beacons)
}

// Controller will send message to each Beacon.
func testCtrlSendToBeacon(t *testing.T, nodes []*node.Node, beacons []*beacon.Beacon) {
	const (
		goroutines = 128
		times      = 64
	)
	ctx := context.Background()
	send := func(start int, guid *guid.GUID) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send with deflate %d", i))
			err := ctrl.SendToBeacon(ctx, guid, messages.CMDBTest, msg, true)
			require.NoError(t, err)
			msg = []byte(fmt.Sprintf("test send without deflate %d", i))
			err = ctrl.SendToBeacon(ctx, guid, messages.CMDBTest, msg, false)
			require.NoError(t, err)
		}
	}

	wg := sync.WaitGroup{}
	sendAndRead := func(n int, beacon *beacon.Beacon) {
		defer wg.Done()
		// send
		for i := 0; i < goroutines; i++ {
			go send(i*times, beacon.GUID())
		}
		// read
		recv := make(map[string]struct{}, 2*goroutines*times)
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		for i := 0; i < 2*goroutines*times; i++ {
			timer.Reset(3 * time.Second)
			select {
			case msg := <-beacon.Test.SendTestMsg:
				recv[string(msg)] = struct{}{}
			case <-timer.C:
				format := "read beacon[%d].Test.SendTestMsg timeout i: %d"
				t.Fatalf(format, n, i)
			}
		}
		select {
		case <-beacon.Test.SendTestMsg:
			t.Fatalf(" read beacon[%d] redundancy send", n)
		case <-time.After(time.Second):
		}
		for i := 0; i < goroutines*times; i++ {
			format := "beacon[%d] lost %s"
			withDeflate := fmt.Sprintf("test send with deflate %d", i)
			_, ok := recv[withDeflate]
			require.Truef(t, ok, format, n, withDeflate)
			withoutDeflate := fmt.Sprintf("test send without deflate %d", i)
			_, ok = recv[withoutDeflate]
			require.Truef(t, ok, format, n, withoutDeflate)
		}
	}
	// send and read
	for i := 0; i < len(beacons); i++ {
		wg.Add(1)
		go sendAndRead(i, beacons[i])
	}
	wg.Wait()

	// clean
	for i := 0; i < len(beacons); i++ {
		Beacon := beacons[i]
		beacons[i] = nil
		beaconGUID := Beacon.GUID()

		Beacon.Exit(nil)
		testsuite.IsDestroyed(t, Beacon)

		err := ctrl.DeleteBeaconUnscoped(beaconGUID)
		require.NoError(t, err)
	}
	for i := 0; i < len(nodes); i++ {
		Node := nodes[i]
		nodes[i] = nil
		nodeGUID := Node.GUID()

		Node.Exit(nil)
		testsuite.IsDestroyed(t, Node)

		err := ctrl.DeleteNodeUnscoped(nodeGUID)
		require.NoError(t, err)
	}
}

func TestNode_Send_CI(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithCI(t, false)
	testNodeSend(t, iNodes, cNodes)
}

func TestNode_Send_IC(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithIC(t, false)
	testNodeSend(t, iNodes, cNodes)
}

func TestNode_Send_Mix(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithMix(t, false)
	testNodeSend(t, iNodes, cNodes)
}

// Each Node will send message to the Controller.
func testNodeSend(t *testing.T, iNodes, cNodes []*node.Node) {
	ctrl.Test.EnableRoleSendTestMessage()

	const (
		goroutines = 128
		times      = 64
	)
	ctx := context.Background()
	send := func(start int, node *node.Node) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send with deflate %d", i))
			err := node.Send(ctx, messages.CMDBTest, msg, true)
			require.NoError(t, err)
			msg = []byte(fmt.Sprintf("test send without deflate %d", i))
			err = node.Send(ctx, messages.CMDBTest, msg, false)
			require.NoError(t, err)
		}
	}

	wg := sync.WaitGroup{}
	sendAndRead := func(n int, node *node.Node, initial bool) {
		defer wg.Done()
		ch := ctrl.Test.CreateNodeSendTestMessageChannel(node.GUID())

		var prefix string
		if initial {
			prefix = "Initial Node[%d]"
		} else {
			prefix = "Common Node[%d]"
		}
		// send
		for i := 0; i < goroutines; i++ {
			go send(i*times, node)
		}
		// read
		recv := make(map[string]struct{}, 2*goroutines*times)
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		for i := 0; i < 2*goroutines*times; i++ {
			timer.Reset(3 * time.Second)
			select {
			case msg := <-ch:
				recv[string(msg)] = struct{}{}
			case <-timer.C:
				format := "read " + prefix + " channel timeout i: %d"
				t.Fatalf(format, n, i)
			}
		}
		select {
		case <-ch:
			t.Fatalf("read "+prefix+" redundancy send", n)
		case <-time.After(time.Second):
		}
		for i := 0; i < goroutines*times; i++ {
			format := "lost " + prefix + " %s"
			withDeflate := fmt.Sprintf("test send with deflate %d", i)
			_, ok := recv[withDeflate]
			require.Truef(t, ok, format, n, withDeflate)
			withoutDeflate := fmt.Sprintf("test send without deflate %d", i)
			_, ok = recv[withoutDeflate]
			require.Truef(t, ok, format, n, withoutDeflate)
		}
	}
	// send and read
	for i := 0; i < len(iNodes); i++ {
		wg.Add(1)
		go sendAndRead(i, iNodes[i], true)
	}
	for i := 0; i < len(cNodes); i++ {
		wg.Add(1)
		go sendAndRead(i, cNodes[i], false)
	}
	wg.Wait()

	// clean
	for i := 0; i < len(cNodes); i++ {
		cNode := cNodes[i]
		cNodes[i] = nil
		cNodeGUID := cNode.GUID()

		cNode.Exit(nil)
		testsuite.IsDestroyed(t, cNode)

		err := ctrl.DeleteNodeUnscoped(cNodeGUID)
		require.NoError(t, err)
	}
	for i := 0; i < len(iNodes); i++ {
		iNode := iNodes[i]
		iNodes[i] = nil
		iNodeGUID := iNode.GUID()

		iNode.Exit(nil)
		testsuite.IsDestroyed(t, iNode)

		err := ctrl.DeleteNodeUnscoped(iNodeGUID)
		require.NoError(t, err)
	}
}

func TestBeacon_Send_CI(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithCI(t, false)
	testBeaconSend(t, nodes, beacons)
}

func TestBeacon_Send_IC(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithIC(t, false)
	testBeaconSend(t, nodes, beacons)
}

func TestBeacon_Send_Mix(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithMix(t, false)
	testBeaconSend(t, nodes, beacons)
}

func testBeaconSend(t *testing.T, nodes []*node.Node, beacons []*beacon.Beacon) {
	ctrl.Test.EnableRoleSendTestMessage()

	const (
		goroutines = 128
		times      = 64
	)
	ctx := context.Background()
	send := func(start int, beacon *beacon.Beacon) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send with deflate %d", i))
			err := beacon.Send(ctx, messages.CMDBTest, msg, true)
			require.NoError(t, err)
			msg = []byte(fmt.Sprintf("test send without deflate %d", i))
			err = beacon.Send(ctx, messages.CMDBTest, msg, false)
			require.NoError(t, err)
		}
	}

	wg := sync.WaitGroup{}
	sendAndRead := func(n int, beacon *beacon.Beacon) {
		defer wg.Done()
		ch := ctrl.Test.CreateBeaconSendTestMessageChannel(beacon.GUID())

		// send
		for i := 0; i < goroutines; i++ {
			go send(i*times, beacon)
		}
		// read
		recv := make(map[string]struct{}, 2*goroutines*times)
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		for i := 0; i < 2*goroutines*times; i++ {
			timer.Reset(3 * time.Second)
			select {
			case msg := <-ch:
				recv[string(msg)] = struct{}{}
			case <-timer.C:
				format := "read beacon[%d] channel timeout i: %d"
				t.Fatalf(format, n, i)
			}
		}
		select {
		case <-ch:
			t.Fatalf(" read beacon[%d] redundancy send", n)
		case <-time.After(time.Second):
		}
		for i := 0; i < goroutines*times; i++ {
			format := "lost beacon[%d] %s"
			withDeflate := fmt.Sprintf("test send with deflate %d", i)
			_, ok := recv[withDeflate]
			require.Truef(t, ok, format, n, withDeflate)
			withoutDeflate := fmt.Sprintf("test send without deflate %d", i)
			_, ok = recv[withoutDeflate]
			require.Truef(t, ok, format, n, withoutDeflate)
		}
	}
	// send and read
	for i := 0; i < len(beacons); i++ {
		wg.Add(1)
		go sendAndRead(i, beacons[i])
	}
	wg.Wait()

	// clean
	for i := 0; i < len(beacons); i++ {
		Beacon := beacons[i]
		beacons[i] = nil
		beaconGUID := Beacon.GUID()

		Beacon.Exit(nil)
		testsuite.IsDestroyed(t, Beacon)

		err := ctrl.DeleteBeaconUnscoped(beaconGUID)
		require.NoError(t, err)
	}
	for i := 0; i < len(nodes); i++ {
		Node := nodes[i]
		nodes[i] = nil
		nodeGUID := Node.GUID()

		Node.Exit(nil)
		testsuite.IsDestroyed(t, Node)

		err := ctrl.DeleteNodeUnscoped(nodeGUID)
		require.NoError(t, err)
	}
}

func TestBeacon_Query_CI(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithCI(t, true)
	testBeaconQuery(t, nodes, beacons)
}

func TestBeacon_Query_IC(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithIC(t, true)
	testBeaconQuery(t, nodes, beacons)
}

func TestBeacon_Query_Mix(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithMix(t, true)
	testBeaconQuery(t, nodes, beacons)
}

func testBeaconQuery(t *testing.T, nodes []*node.Node, beacons []*beacon.Beacon) {

	// clean
	for i := 0; i < len(beacons); i++ {
		Beacon := beacons[i]
		beacons[i] = nil
		beaconGUID := Beacon.GUID()

		Beacon.Exit(nil)
		testsuite.IsDestroyed(t, Beacon)

		err := ctrl.DeleteBeaconUnscoped(beaconGUID)
		require.NoError(t, err)
	}
	for i := 0; i < len(nodes); i++ {
		Node := nodes[i]
		nodes[i] = nil
		nodeGUID := Node.GUID()

		Node.Exit(nil)
		testsuite.IsDestroyed(t, Node)

		err := ctrl.DeleteNodeUnscoped(nodeGUID)
		require.NoError(t, err)
	}
}

func TestNodeQueryRoleKey(t *testing.T) {
	iNode := generateInitialNodeAndTrust(t, 0)
	iNodeGUID := iNode.GUID()

	// create bootstrap
	iListener, err := iNode.GetListener(initialNodeListenerTag)
	require.NoError(t, err)
	iAddr := iListener.Addr()
	bListener := &bootstrap.Listener{
		Mode:    iListener.Mode(),
		Network: iAddr.Network(),
		Address: iAddr.String(),
	}

	ctrl.Test.CreateNodeRegisterRequestChannel()
	ctrl.Test.CreateBeaconRegisterRequestChannel()

	// create and run Beacon
	beaconCfg := generateBeaconConfig(t, "Beacon")
	// must copy, because Beacon register will cover bytes
	boot, key := generateBootstrap(t, bListener)
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

	// create and run Common Node
	cNodeCfg := generateNodeConfig(t, "Common Node")
	boot, key = generateBootstrap(t, bListener)
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
		err = ctrl.AcceptRegisterNode(nrr, nil, false)
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("read Ctrl.Test.NodeRegisterRequest timeout")
	}
	timer = time.AfterFunc(10*time.Second, func() {
		t.Fatal("node register timeout")
	})
	cNode.Wait()
	timer.Stop()

	// Common Node synchronize with Initial Node
	err = cNode.Synchronize(context.Background(), iNodeGUID, bListener)
	require.NoError(t, err)
	// Common Node add Listener
	err = cNode.AddListener(&messages.Listener{
		Tag:     "test",
		Mode:    xnet.ModeTCP,
		Network: "tcp",
		Address: "127.0.0.1:0",
	})
	require.NoError(t, err)
	cListener, err := cNode.GetListener("test")
	require.NoError(t, err)
	cNodeGUID := cNode.GUID()

	// Beacon synchronize with Common Node
	targetListener := bootstrap.Listener{
		Mode:    xnet.ModeTCP,
		Network: cListener.Addr().Network(),
		Address: cListener.Addr().String(),
	}
	err = Beacon.Synchronize(context.Background(), cNodeGUID, &targetListener)
	require.NoError(t, err)

	// Beacon send messages
	beaconGUID := Beacon.GUID()
	Beacon.Test.EnableTestMessage()
	ctrl.EnableInteractiveMode(beaconGUID)

	const (
		goroutines = 256
		times      = 64
	)
	ctx := context.Background()
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := ctrl.SendToBeacon(ctx, beaconGUID, messages.CMDBTest, msg, true)
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
	recv.Grow(8 << 20)
	timer = time.NewTimer(3 * time.Second)
	for i := 0; i < goroutines*times; i++ {
		timer.Reset(3 * time.Second)
		select {
		case b := <-Beacon.Test.SendTestMsg:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read beacon channel timeout i: %d", i)
		}
	}
	select {
	case <-Beacon.Test.SendTestMsg:
		t.Fatal("redundancy send")
	case <-time.After(time.Second):
	}
	str := recv.String()
	for i := 0; i < goroutines*times; i++ {
		need := fmt.Sprintf("test send %d", i)
		require.True(t, strings.Contains(str, need), "lost: %s", need)
	}

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

func TestBeacon_Query(t *testing.T) {
	iNode := generateInitialNodeAndTrust(t, 0)
	iNodeGUID := iNode.GUID()

	// create bootstrap
	iListener, err := iNode.GetListener(initialNodeListenerTag)
	require.NoError(t, err)
	iAddr := iListener.Addr()
	bListener := &bootstrap.Listener{
		Mode:    iListener.Mode(),
		Network: iAddr.Network(),
		Address: iAddr.String(),
	}
	boot, key := generateBootstrap(t, bListener)
	ctrl.Test.CreateBeaconRegisterRequestChannel()

	// create and run Beacon
	beaconCfg := generateBeaconConfig(t, "Beacon")
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

	// connect Initial Node
	err = Beacon.Synchronize(context.Background(), iNodeGUID, bListener)
	require.NoError(t, err)

	// Controller send message
	beaconGUID := Beacon.GUID()
	Beacon.Test.EnableTestMessage()

	const (
		goroutines = 8
		times      = 32
	)
	ctx := context.Background()
	send := func(prefix string) {
		for i := 0; i < times; i++ {
			msg := []byte(fmt.Sprintf("test send %s%d", prefix, i))
			err := ctrl.SendToBeacon(ctx, beaconGUID, messages.CMDBTest, msg, true)
			if err != nil {
				t.Error(err)
				return
			}
		}
	}
	for i := 0; i < goroutines; i++ {
		go send(strconv.Itoa(i))
	}
	time.Sleep(time.Second)
	// Beacon Query loop
	go func() {
		for i := 0; i < goroutines*times; i++ {
			err := Beacon.Query()
			if err != nil {
				t.Error(err)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
	recv := bytes.Buffer{}
	recv.Grow(8 << 20)
	timer = time.NewTimer(3 * time.Second)
	for i := 0; i < goroutines*times; i++ {
		timer.Reset(3 * time.Second)
		select {
		case b := <-Beacon.Test.SendTestMsg:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read Beacon.Test.SendTestMsg timeout i: %d", i)
		}
	}
	select {
	case <-Beacon.Test.SendTestMsg:
		t.Fatal("redundancy send")
	case <-time.After(time.Second):
	}
	str := recv.String()
	for i := 0; i < goroutines; i++ {
		for j := 0; j < times; j++ {
			need := fmt.Sprintf("test send %d%d", i, j)
			require.True(t, strings.Contains(str, need), "lost: %s", need)
		}
	}

	// clean
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)
	Beacon.Exit(nil)
	testsuite.IsDestroyed(t, Beacon)

	err = ctrl.DeleteBeaconUnscoped(beaconGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(iNodeGUID)
	require.NoError(t, err)
}
