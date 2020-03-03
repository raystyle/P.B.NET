package test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/guid"
	"project/internal/messages"
	"project/internal/testsuite"

	"project/beacon"
	"project/node"
)

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
func buildBeaconNetworkWithCI(
	t *testing.T,
	recvMsg bool,
	interactive bool,
) ([]*node.Node, []*beacon.Beacon) {
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
		if interactive {
			ctrl.EnableInteractiveMode(Beacon.GUID())
		}

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
func buildBeaconNetworkWithIC(
	t *testing.T,
	recvMsg bool,
	interactive bool,
) ([]*node.Node, []*beacon.Beacon) {
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
		if interactive {
			ctrl.EnableInteractiveMode(Beacon.GUID())
		}

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
func buildBeaconNetworkWithMix(
	t *testing.T,
	recvMsg bool,
	interactive bool,
) ([]*node.Node, []*beacon.Beacon) {
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
	if interactive {
		ctrl.EnableInteractiveMode(beacons[0].GUID())
	}

	// Beacon 1 connect the Common Node 0 and the Common Node 2
	beacons[1] = generateBeacon(t, cNodes[0], commonNodeListenerTag, 1)
	err = beacons[1].Synchronize(ctx, c0GUID, c0Listener)
	require.NoError(t, err)
	err = beacons[1].Synchronize(ctx, c2GUID, c2Listener)
	require.NoError(t, err)
	if recvMsg {
		beacons[1].Test.EnableTestMessage()
	}
	if interactive {
		ctrl.EnableInteractiveMode(beacons[1].GUID())
	}

	// Beacon 2 connect the Common Node 1 and the Common Node 2
	beacons[2] = generateBeacon(t, cNodes[1], commonNodeListenerTag, 2)
	err = beacons[2].Synchronize(ctx, c1GUID, c1Listener)
	require.NoError(t, err)
	err = beacons[2].Synchronize(ctx, c2GUID, c2Listener)
	require.NoError(t, err)
	if recvMsg {
		beacons[2].Test.EnableTestMessage()
	}
	if interactive {
		ctrl.EnableInteractiveMode(beacons[2].GUID())
	}

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
		timer := time.NewTimer(senderTimeout)
		defer timer.Stop()
		for i := 0; i < 2*goroutines*times; i++ {
			timer.Reset(senderTimeout)
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
		timer := time.NewTimer(senderTimeout)
		defer timer.Stop()
		for i := 0; i < 2*goroutines*times; i++ {
			timer.Reset(senderTimeout)
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
	nodes, beacons := buildBeaconNetworkWithCI(t, true, true)
	testCtrlSendToBeacon(t, nodes, beacons)
}

func TestCtrl_SendToBeacon_IC(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithIC(t, true, true)
	testCtrlSendToBeacon(t, nodes, beacons)
}

func TestCtrl_SendToBeacon_Mix(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithMix(t, true, true)
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
		timer := time.NewTimer(senderTimeout)
		defer timer.Stop()
		for i := 0; i < 2*goroutines*times; i++ {
			timer.Reset(senderTimeout)
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
		timer := time.NewTimer(senderTimeout)
		defer timer.Stop()
		for i := 0; i < 2*goroutines*times; i++ {
			timer.Reset(senderTimeout)
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
	nodes, beacons := buildBeaconNetworkWithCI(t, false, true)
	testBeaconSend(t, nodes, beacons)
}

func TestBeacon_Send_IC(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithIC(t, false, true)
	testBeaconSend(t, nodes, beacons)
}

func TestBeacon_Send_Mix(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithMix(t, false, true)
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
		timer := time.NewTimer(senderTimeout)
		defer timer.Stop()
		for i := 0; i < 2*goroutines*times; i++ {
			timer.Reset(senderTimeout)
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
	nodes, beacons := buildBeaconNetworkWithCI(t, true, false)
	testBeaconQuery(t, nodes, beacons)
}

func TestBeacon_Query_IC(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithIC(t, true, false)
	testBeaconQuery(t, nodes, beacons)
}

func TestBeacon_Query_Mix(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithMix(t, true, false)
	testBeaconQuery(t, nodes, beacons)
}

func testBeaconQuery(t *testing.T, nodes []*node.Node, beacons []*beacon.Beacon) {
	const times = 128

	ctx := context.Background()

	wg := sync.WaitGroup{}
	sendAndRead := func(n int, beacon *beacon.Beacon) {
		defer wg.Done()
		beaconGUID := beacon.GUID()

		timer := time.NewTimer(senderTimeout)
		defer timer.Stop()

		for i := 0; i < times; i++ {
			// send(message will be saved to the database)
			msg := []byte(fmt.Sprintf("test send with deflate %d", i))
			err := ctrl.SendToBeacon(ctx, beaconGUID, messages.CMDBTest, msg, true)
			require.NoError(t, err)
			msg = []byte(fmt.Sprintf("test send without deflate %d", i))
			err = ctrl.SendToBeacon(ctx, beaconGUID, messages.CMDBTest, msg, false)
			require.NoError(t, err)

			// wait Beacon query
			format := "beacon[%d] lost %s"
			// with deflate
			err = beacon.Query()
			require.NoError(t, err)
			timer.Reset(senderTimeout)
			select {
			case msg := <-beacon.Test.SendTestMsg:
				withDeflate := fmt.Sprintf("test send with deflate %d", i)
				require.Equalf(t, withDeflate, string(msg), format, n, withDeflate)
			case <-timer.C:
				format := "read beacon[%d].Test.SendTestMsg timeout i: %d"
				t.Fatalf(format, n, i)
			}
			// without deflate
			err = beacon.Query()
			require.NoError(t, err)
			timer.Reset(senderTimeout)
			select {
			case msg := <-beacon.Test.SendTestMsg:
				withoutDeflate := fmt.Sprintf("test send without deflate %d", i)
				require.Equalf(t, withoutDeflate, string(msg), format, n, withoutDeflate)
			case <-timer.C:
				format := "read beacon[%d].Test.SendTestMsg timeout i: %d"
				t.Fatalf(format, n, i)
			}
		}
	}

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
