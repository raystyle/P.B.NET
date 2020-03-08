package test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/guid"
	"project/internal/messages"
	"project/internal/testsuite"

	"project/beacon"
	"project/node"
)

func TestCtrl_SendToNodeRT_CI(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithCI(t, false)
	testCtrlSendToNodeRT(t, iNodes, cNodes)
}

func TestCtrl_SendToNodeRT_IC(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithIC(t, false)
	testCtrlSendToNodeRT(t, iNodes, cNodes)
}

func TestCtrl_SendToNodeRT_Mix(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithMix(t, false)
	testCtrlSendToNodeRT(t, iNodes, cNodes)
}

// Controller will send message to each Node.
func testCtrlSendToNodeRT(t *testing.T, iNodes, cNodes []*node.Node) {
	const (
		goroutines = 128
		times      = 32
	)
	ctx := context.Background()
	send := func(wg *sync.WaitGroup, info string, start int, guid *guid.GUID) {
		defer wg.Done()
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test request with deflate %d", i))
			req := &messages.TestRequest{
				Request: msg,
			}
			resp, err := ctrl.SendToNodeRT(ctx, guid,
				messages.CMDBRTTestRequest, req, true)
			require.NoError(t, err, info)
			response := resp.(*messages.TestResponse).Response
			require.Equal(t, msg, response)
		}
	}

	wg := sync.WaitGroup{}
	sendAndWait := func(n int, node *node.Node, initial bool) {
		defer wg.Done()
		var info string
		if initial {
			info = fmt.Sprintf("Initial Node[%d]", n)
		} else {
			info = fmt.Sprintf("Common Node[%d]", n)
		}
		// send
		sub := new(sync.WaitGroup)
		for i := 0; i < goroutines; i++ {
			sub.Add(1)
			go send(sub, info, i*times, node.GUID())
		}
		sub.Wait()
	}
	// send and read
	for i := 0; i < len(iNodes); i++ {
		wg.Add(1)
		go sendAndWait(i, iNodes[i], true)
	}
	for i := 0; i < len(cNodes); i++ {
		wg.Add(1)
		go sendAndWait(i, cNodes[i], false)
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

func TestCtrl_SendToBeaconRT_CI(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithCI(t, false, true)
	testCtrlSendToBeaconRT(t, nodes, beacons)
}

func TestCtrl_SendToBeaconRT_IC(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithIC(t, false, true)
	testCtrlSendToBeaconRT(t, nodes, beacons)
}

func TestCtrl_SendToBeaconRT_Mix(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithMix(t, false, true)
	testCtrlSendToBeaconRT(t, nodes, beacons)
}

// Controller will send message to each Beacon.
func testCtrlSendToBeaconRT(t *testing.T, nodes []*node.Node, beacons []*beacon.Beacon) {
	const (
		goroutines = 128
		times      = 32
	)
	ctx := context.Background()
	send := func(wg *sync.WaitGroup, info string, start int, guid *guid.GUID) {
		defer wg.Done()
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test request with deflate %d", i))
			req := &messages.TestRequest{
				Request: msg,
			}
			resp, err := ctrl.SendToBeaconRT(ctx, guid,
				messages.CMDBRTTestRequest, req, true)
			require.NoError(t, err, info)
			response := resp.(*messages.TestResponse).Response
			require.Equal(t, msg, response)
		}
	}

	wg := sync.WaitGroup{}
	sendAndWait := func(n int, beacon *beacon.Beacon) {
		defer wg.Done()
		sub := new(sync.WaitGroup)
		for i := 0; i < goroutines; i++ {
			info := fmt.Sprintf("Beacon[%d]", n)
			sub.Add(1)
			go send(sub, info, i*times, beacon.GUID())
		}
		sub.Wait()
	}
	// send and read
	for i := 0; i < len(beacons); i++ {
		wg.Add(1)
		go sendAndWait(i, beacons[i])
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

func TestNode_SendRT_CI(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithCI(t, false)
	testNodeSendRT(t, iNodes, cNodes)
}

func TestNode_SendRT_IC(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithIC(t, false)
	testNodeSendRT(t, iNodes, cNodes)
}

func TestNode_SendRT_Mix(t *testing.T) {
	iNodes, cNodes := buildNodeNetworkWithMix(t, false)
	testNodeSendRT(t, iNodes, cNodes)
}

// Each Node will send message to Controller.
func testNodeSendRT(t *testing.T, iNodes, cNodes []*node.Node) {
	const (
		goroutines = 128
		times      = 32
	)
	ctx := context.Background()
	send := func(wg *sync.WaitGroup, info string, start int, node *node.Node) {
		defer wg.Done()
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test request with deflate %d", i))
			req := &messages.TestRequest{
				Request: msg,
			}
			resp, err := node.SendRT(ctx, messages.CMDBRTTestRequest, req, true)
			require.NoError(t, err, info)
			response := resp.(*messages.TestResponse).Response
			require.Equal(t, msg, response)
		}
	}

	wg := sync.WaitGroup{}
	sendAndWait := func(n int, node *node.Node, initial bool) {
		defer wg.Done()
		var info string
		if initial {
			info = fmt.Sprintf("Initial Node[%d]", n)
		} else {
			info = fmt.Sprintf("Common Node[%d]", n)
		}
		// send
		sub := new(sync.WaitGroup)
		for i := 0; i < goroutines; i++ {
			sub.Add(1)
			go send(sub, info, i*times, node)
		}
		sub.Wait()
	}
	// send and read
	for i := 0; i < len(iNodes); i++ {
		wg.Add(1)
		go sendAndWait(i, iNodes[i], true)
	}
	for i := 0; i < len(cNodes); i++ {
		wg.Add(1)
		go sendAndWait(i, cNodes[i], false)
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

func TestBeacon_SendRT_CI(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithCI(t, false, true)
	testBeaconSendRT(t, nodes, beacons)
}

func TestBeacon_SendRT_IC(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithIC(t, false, true)
	testBeaconSendRT(t, nodes, beacons)
}

func TestBeacon_SendRT_Mix(t *testing.T) {
	nodes, beacons := buildBeaconNetworkWithMix(t, false, true)
	testBeaconSendRT(t, nodes, beacons)
}

func testBeaconSendRT(t *testing.T, nodes []*node.Node, beacons []*beacon.Beacon) {
	const (
		goroutines = 128
		times      = 32
	)
	ctx := context.Background()
	send := func(wg *sync.WaitGroup, info string, start int, beacon *beacon.Beacon) {
		defer wg.Done()
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test request with deflate %d", i))
			req := &messages.TestRequest{
				Request: msg,
			}
			resp, err := beacon.SendRT(ctx, messages.CMDBRTTestRequest, req, true)
			require.NoError(t, err, info)
			response := resp.(*messages.TestResponse).Response
			require.Equal(t, msg, response)
		}
	}

	wg := sync.WaitGroup{}
	sendAndRead := func(n int, beacon *beacon.Beacon) {
		defer wg.Done()
		sub := new(sync.WaitGroup)
		for i := 0; i < goroutines; i++ {
			info := fmt.Sprintf("Beacon[%d]", n)
			sub.Add(1)
			go send(sub, info, i*times, beacon)
		}
		sub.Wait()
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
