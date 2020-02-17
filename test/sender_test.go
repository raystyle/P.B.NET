package test

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
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

func TestAll(t *testing.T) {
	TestCtrl_SendToNode_PassInitialNode(t)
	TestNode_Send_PassInitialNode(t)
	TestCtrl_SendToBeacon_PassICNodes(t)
	TestBeacon_Send_PassCommonNode(t)
	TestNodeQueryRoleKey(t)
}

func TestAll_Parallel(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(5)
	go func() {
		defer wg.Done()
		TestCtrl_SendToNode_PassInitialNode(t)
		fmt.Println("test-a")
	}()
	go func() {
		defer wg.Done()
		TestNode_Send_PassInitialNode(t)
		fmt.Println("test-b")
	}()
	go func() {
		defer wg.Done()
		TestCtrl_SendToBeacon_PassICNodes(t)
		fmt.Println("test-c")
	}()
	go func() {
		defer wg.Done()
		TestBeacon_Send_PassCommonNode(t)
		fmt.Println("test-d")
	}()
	go func() {
		defer wg.Done()
		TestNodeQueryRoleKey(t)
		fmt.Println("test-e")
	}()
	wg.Wait()
}

func TestLoop(t *testing.T) {
	// t.Skip("must run it manually")
	logLevel = "warning"
	for i := 0; i < 100; i++ {
		fmt.Println("round:", i+1)
		TestAll(t)
		time.Sleep(2 * time.Second)
		runtime.GC()
		debug.FreeOSMemory()
		time.Sleep(10 * time.Second)
		runtime.GC()
		debug.FreeOSMemory()
		time.Sleep(5 * time.Second)
	}
}

func TestLoop_Parallel(t *testing.T) {
	// t.Skip("must run it manually")
	logLevel = "warning"
	for i := 0; i < 100; i++ {
		fmt.Println("round:", i+1)
		TestAll_Parallel(t)
		time.Sleep(2 * time.Second)
		runtime.GC()
		debug.FreeOSMemory()
		time.Sleep(10 * time.Second)
		runtime.GC()
		debug.FreeOSMemory()
		time.Sleep(5 * time.Second)
	}
}

// 3 * (A Common Node Connect the Initial Node)
//
//  +------------+     +----------------+     +---------------+
//  |            | --> | Initial Node 0 | <-- | Common Node 0 |
//  |            |     +----------------+     +---------------+
//  |            |
//  |            |     +----------------+     +---------------+
//  | Controller | --> | Initial Node 1 | <-- | Common Node 1 |
//  |            |     +----------------+     +---------------+
//  |            |
//  |            |     +----------------+     +---------------+
//  |            | --> | Initial Node 2 | <-- | Common Node 2 |
//  +------------+     +----------------+     +---------------+
//
func TestCtrl_Broadcast_CI(t *testing.T) {
	const num = 3
	var (
		iNodes [num]*node.Node
		cNodes [num]*node.Node
	)
	// connect
	for i := 0; i < num; i++ {
		iNode, bListener, cNode := generateInitialNodeAndCommonNode(t, i, i)

		iNode.Test.EnableTestMessage()
		cNode.Test.EnableTestMessage()

		// try to connect Initial Node and start to synchronize
		err := cNode.Synchronize(context.Background(), iNode.GUID(), bListener)
		require.NoError(t, err)

		iNodes[i] = iNode
		cNodes[i] = cNode
	}
	testCtrlBroadcast(t, iNodes[:], cNodes[:])
}

// 3 * (Initial Node connect the Common Node Connect)
//
//  +------------+     +----------------+
//  |            | --> | Initial Node 0 |
//  |            |     +----------------+
//  |            |             ↓
//  |            |     +---------------+
//  |            | --> | Common Node 0 |
//  |            |     +---------------+
//  |            |
//  |            |     +----------------+
//  |            | --> | Initial Node 1 |
//  |            |     +----------------+
//  | Controller |             ↓
//  |            |     +---------------+
//  |            | --> | Common Node 1 |
//  |            |     +---------------+
//  |            |
//  |            |     +----------------+
//  |            | --> | Initial Node 2 |
//  |            |     +----------------+
//  |            |             ↓
//  |            |     +---------------+
//  |            | --> | Common Node 2 |
//  +------------+     +---------------+
//
func TestCtrl_Broadcast_IC(t *testing.T) {
	const num = 3
	var (
		iNodes [num]*node.Node
		cNodes [num]*node.Node
	)
	// connect
	for i := 0; i < num; i++ {
		iNode, _, cNode := generateInitialNodeAndCommonNode(t, i, i)

		iNode.Test.EnableTestMessage()
		cNode.Test.EnableTestMessage()

		// add listener to Common Node
		const listenerTag = "test"
		listener := &messages.Listener{
			Tag:     listenerTag,
			Mode:    xnet.ModeTCP,
			Network: "tcp",
			Address: "localhost:0",
		}
		err := cNode.AddListener(listener)
		require.NoError(t, err)
		dListener, err := cNode.GetListener(listenerTag)
		require.NoError(t, err)
		bListener := &bootstrap.Listener{
			Mode:    xnet.ModeTCP,
			Network: "tcp",
			Address: dListener.Addr().String(),
		}

		// Controller must connect the Common Node, otherwise the Common Node
		// can't query Node key from Controller
		err = ctrl.Synchronize(context.Background(), cNode.GUID(), bListener)
		require.NoError(t, err)

		// Initial Node connect the Common Node and start to synchronize
		err = iNode.Synchronize(context.Background(), cNode.GUID(), bListener)
		require.NoError(t, err)

		iNodes[i] = iNode
		cNodes[i] = cNode
	}
	testCtrlBroadcast(t, iNodes[:], cNodes[:])
}

func testCtrlBroadcast(t *testing.T, iNodes, cNodes []*node.Node) {
	const (
		goroutines = 32
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
	for i := 0; i < goroutines; i++ {
		go broadcast(i * times)
	}

	read := func(n int, wg *sync.WaitGroup, Node *node.Node, initial bool) {
		defer wg.Done()
		var prefix string
		if initial {
			prefix = "Initial Node[%d]"
		} else {
			prefix = "Common Node[%d]"
		}
		recv := bytes.Buffer{}
		recv.Grow(1 << 20)
		timer := time.NewTimer(3 * time.Second)
		for i := 0; i < 2*goroutines*times; i++ {
			timer.Reset(3 * time.Second)
			select {
			case b := <-Node.Test.BroadcastTestMsg:
				recv.Write(b)
				recv.WriteString("\n")
			case <-timer.C:
				format := "read " + prefix + ".Test.BroadcastTestMsg timeout i: %d"
				t.Fatalf(format, n, i)
			}
		}
		select {
		case <-Node.Test.BroadcastTestMsg:
			t.Fatal("redundancy broadcast")
		case <-time.After(time.Second):
		}
		str := recv.String()
		for i := 0; i < goroutines*times; i++ {
			format := prefix + "lost: %s"
			withDeflate := fmt.Sprintf("test broadcast with deflate %d", i)
			require.Truef(t, strings.Contains(str, withDeflate), format, n, withDeflate)
			withoutDeflate := fmt.Sprintf("test broadcast without deflate %d", i)
			require.Truef(t, strings.Contains(str, withoutDeflate), format, n, withoutDeflate)
		}
	}

	// read message
	wg := new(sync.WaitGroup)
	for i := 0; i < len(iNodes); i++ {
		wg.Add(1)
		go read(i, wg, iNodes[i], true)

	}
	for i := 0; i < len(cNodes); i++ {
		wg.Add(1)
		go read(i, wg, cNodes[i], false)
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

// One Common Node connect the Initial Node
// Controller connect the Initial Node
// Controller send test messages
func TestCtrl_SendToNode_PassInitialNode(t *testing.T) {
	iNode := generateInitialNodeAndTrust(t, 0)
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

	// create and run Common Node
	cNodeCfg := generateNodeConfig(t, "Common Node")
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

	// wait common node
	timer := time.AfterFunc(10*time.Second, func() {
		t.Fatal("node register timeout")
	})
	cNode.Wait()
	timer.Stop()

	// try to connect initial node
	err = cNode.Synchronize(context.Background(), iNode.GUID(), bListener)
	require.NoError(t, err)

	// controller send messages
	cNodeGUID := cNode.GUID()
	cNode.Test.EnableTestMessage()

	const (
		goroutines = 256
		times      = 64
	)
	ctx := context.Background()
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := ctrl.SendToNode(ctx, cNodeGUID, messages.CMDBTest, msg, true)
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
		case b := <-cNode.Test.SendTestMsg:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read Node.Test.SendTestMsg timeout i: %d", i)
		}
	}
	select {
	case <-cNode.Test.SendTestMsg:
		t.Fatal("redundancy send")
	case <-time.After(time.Second):
	}
	str := recv.String()
	for i := 0; i < goroutines*times; i++ {
		need := fmt.Sprintf("test send %d", i)
		require.True(t, strings.Contains(str, need), "lost: %s", need)
	}

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

// One Beacon connect the Initial Node, Controller connect the Initial Node,
// Controller send test messages to Beacon in interactive mode.
func TestCtrl_SendToBeacon_PassInitialNode(t *testing.T) {
	iNode := generateInitialNodeAndTrust(t, 0)
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
	ctrl.Test.CreateBeaconRegisterRequestChannel()

	// create Beacon
	beaconCfg := generateBeaconConfig(t, "Beacon")
	beaconCfg.Register.FirstBoot = boot
	beaconCfg.Register.FirstKey = key

	// run Beacon
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

	// try to connect initial node
	err = Beacon.Synchronize(context.Background(), iNodeGUID, bListener)
	require.NoError(t, err)

	// controller send messages
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
			t.Fatalf("read Beacon.Test.SendTestMsg timeout i: %d", i)
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
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)

	err = ctrl.DeleteBeaconUnscoped(beaconGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(iNodeGUID)
	require.NoError(t, err)
}

func TestNode_SendDirectly(t *testing.T) {
	Node := generateInitialNodeAndTrust(t, 0)
	NodeGUID := Node.GUID()

	ctrl.Test.EnableRoleSendTestMessage()
	ch := ctrl.Test.CreateNodeSendTestMessageChannel(NodeGUID)

	const (
		goroutines = 256
		times      = 64
	)
	ctx := context.Background()
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := Node.Send(ctx, messages.CMDBTest, msg, true)
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
	timer := time.NewTimer(3 * time.Second)
	for i := 0; i < goroutines*times; i++ {
		timer.Reset(3 * time.Second)
		select {
		case b := <-ch:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read node channel timeout i: %d", i)
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

	err = ctrl.DeleteNodeUnscoped(NodeGUID)
	require.NoError(t, err)
}

// One Common Node connect the Initial Node, Controller connect the Initial Node,
// Node send test messages to Controller
func TestNode_Send_PassInitialNode(t *testing.T) {
	iNode := generateInitialNodeAndTrust(t, 0)
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

	// create and run common node
	cNodeCfg := generateNodeConfig(t, "Common Node")
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

	// wait Common Node
	timer := time.AfterFunc(10*time.Second, func() {
		t.Fatal("node register timeout")
	})
	cNode.Wait()
	timer.Stop()

	// try to connect Initial Node
	err = cNode.Synchronize(context.Background(), iNodeGUID, bListener)
	require.NoError(t, err)

	// controller send messages
	cNodeGUID := cNode.GUID()
	ctrl.Test.EnableRoleSendTestMessage()
	ch := ctrl.Test.CreateNodeSendTestMessageChannel(cNodeGUID)

	const (
		goroutines = 256
		times      = 64
	)
	ctx := context.Background()
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := cNode.Send(ctx, messages.CMDBTest, msg, true)
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
		case b := <-ch:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read node channel timeout i: %d", i)
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
	cNode.Exit(nil)
	testsuite.IsDestroyed(t, cNode)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)

	err = ctrl.DeleteNodeUnscoped(cNodeGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(iNodeGUID)
	require.NoError(t, err)
}

// One Beacon connect the Initial Node, Controller connect the Initial Node,
// Beacon send test messages to Controller in interactive mode.
func TestBeacon_Send_PassInitialNode(t *testing.T) {
	iNode := generateInitialNodeAndTrust(t, 0)
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
	ctrl.Test.CreateBeaconRegisterRequestChannel()

	// create Beacon
	beaconCfg := generateBeaconConfig(t, "Beacon")
	beaconCfg.Register.FirstBoot = boot
	beaconCfg.Register.FirstKey = key

	// run Beacon
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

	// try to connect initial node
	err = Beacon.Synchronize(context.Background(), iNodeGUID, bListener)
	require.NoError(t, err)

	// controller send messages
	beaconGUID := Beacon.GUID()
	ctrl.Test.EnableRoleSendTestMessage()
	ch := ctrl.Test.CreateBeaconSendTestMessageChannel(beaconGUID)

	const (
		goroutines = 256
		times      = 64
	)
	ctx := context.Background()
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := Beacon.Send(ctx, messages.CMDBTest, msg, true)
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
		case b := <-ch:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read beacon channel timeout i: %d", i)
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
	Beacon.Exit(nil)
	testsuite.IsDestroyed(t, Beacon)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)

	err = ctrl.DeleteNodeUnscoped(beaconGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(iNodeGUID)
	require.NoError(t, err)
}

// One Beacon connect the Common Node, Common Node Connect the Initial Node,
// Controller connect the Initial Node, Beacon send test messages to
// Controller in interactive mode.
//
// Beacon -> Common Node -> Initial Node -> Controller
func TestBeacon_Send_PassCommonNode(t *testing.T) {
	iNode := generateInitialNodeAndTrust(t, 0)
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
	ctrl.Test.CreateBeaconRegisterRequestChannel()

	// create and run Common Node
	cNodeCfg := generateNodeConfig(t, "Common Node")
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
	timer := time.AfterFunc(10*time.Second, func() {
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

	// create and run Beacon
	beaconCfg := generateBeaconConfig(t, "Beacon")
	// must copy, because Beacon register will cover bytes
	boot, key = generateBootstrap(t, bListener)
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
	timer = time.AfterFunc(10*time.Second, func() {
		t.Fatal("beacon register timeout")
	})
	Beacon.Wait()
	timer.Stop()

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
	ctrl.Test.EnableRoleSendTestMessage()
	ch := ctrl.Test.CreateBeaconSendTestMessageChannel(beaconGUID)

	const (
		goroutines = 256
		times      = 64
	)
	ctx := context.Background()
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := Beacon.Send(ctx, messages.CMDBTest, msg, true)
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
		case b := <-ch:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read beacon channel timeout i: %d", i)
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

// One Beacon connect the Common Node, Common Node Connect the Initial Node,
// Controller connect the Initial Node, Beacon send test messages to
// Controller in interactive mode.
//
// Controller -> Initial Node -> Common Node -> Beacon
func TestCtrl_SendToBeacon_PassICNodes(t *testing.T) {
	iNode := generateInitialNodeAndTrust(t, 0)
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
	ctrl.Test.CreateBeaconRegisterRequestChannel()

	// create and run Common Node
	cNodeCfg := generateNodeConfig(t, "Common Node")
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
	timer := time.AfterFunc(10*time.Second, func() {
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

	// create and run Beacon
	beaconCfg := generateBeaconConfig(t, "Beacon")
	// must copy, because Beacon register will cover bytes
	boot, key = generateBootstrap(t, bListener)
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
	timer = time.AfterFunc(10*time.Second, func() {
		t.Fatal("beacon register timeout")
	})
	Beacon.Wait()
	timer.Stop()

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

func TestNodeQueryRoleKey(t *testing.T) {
	iNode := generateInitialNodeAndTrust(t, 0)
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
	iListener, err := iNode.GetListener(InitialNodeListenerTag)
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
