package test

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/testsuite"
	"project/internal/xnet"

	"project/beacon"
	"project/node"
)

func TestAll(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(4)
	go func() {
		defer wg.Done()
		TestCtrl_SendToNode_PassInitialNode(t)
		fmt.Println("aaaa")
	}()
	go func() {
		defer wg.Done()
		TestNode_Send_PassInitialNode(t)
		fmt.Println("bbbb")
	}()
	go func() {
		defer wg.Done()
		TestCtrl_SendToBeacon_PassICNodes(t)
		fmt.Println("cccc")
	}()
	go func() {
		defer wg.Done()
		TestBeacon_Send_PassCommonNode(t)
		fmt.Println("dddd")
	}()
	wg.Wait()
}

func TestLoop(t *testing.T) {
	// t.Skip("must run it manually")
	for {
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

// Three Common Node connect the Initial Node
// Controller connect the Initial Node
// Controller broadcast test messages
func TestCtrl_Broadcast_PassInitialNode(t *testing.T) {
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
	ctrl.Test.CreateNodeRegisterRequestChannel()

	// create and run common nodes
	cNodes := make([]*node.Node, 3)
	for i := 0; i < 3; i++ {
		cNodeCfg := generateNodeConfig(t, fmt.Sprintf("Common Node %d", i))
		// must copy, because Node register will cover bytes
		cNodeCfg.Register.FirstBoot = make([]byte, len(boot))
		copy(cNodeCfg.Register.FirstBoot, boot)
		cNodeCfg.Register.FirstKey = make([]byte, len(key))
		copy(cNodeCfg.Register.FirstKey, key)

		cNode, err := node.New(cNodeCfg)
		require.NoError(t, err)
		testsuite.IsDestroyed(t, cNodeCfg)

		cNode.Test.EnableTestMessage()
		cNodes[i] = cNode
		go func() {
			err := cNode.Main()
			require.NoError(t, err)
		}()
	}

	// read node register requests
	for i := 0; i < 3; i++ {
		select {
		case nrr := <-ctrl.Test.NodeRegisterRequest:
			spew.Dump(nrr)
			err = ctrl.AcceptRegisterNode(nrr, false)
			require.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Fatal("read CTRL.Test.NodeRegisterRequest timeout")
		}
	}

	// wait common nodes
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		timer := time.AfterFunc(10*time.Second, func() {
			t.Fatalf("node %d register timeout", i)
		})
		cNodes[i].Wait()
		timer.Stop()

		// connect initial node
		err := cNodes[i].Synchronize(ctx, iNodeGUID, bListener)
		require.NoError(t, err)
	}

	// broadcast
	const (
		goroutines = 64
		times      = 64
	)
	broadcast := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test broadcast %d", i))
			err := ctrl.Broadcast(messages.CMDBTest, msg)
			if err != nil {
				t.Error(err)
				return
			}
		}
	}
	for i := 0; i < goroutines; i++ {
		go broadcast(i * times)
	}

	// read
	wg := sync.WaitGroup{}
	const format = "read Node[%d].Test.BroadcastTestMsg timeout i: %d"
	for n, cNode := range cNodes {
		wg.Add(1)
		go func(n int, Node *node.Node) {
			defer wg.Done()
			recv := bytes.Buffer{}
			recv.Grow(8 << 20)
			timer := time.NewTimer(3 * time.Second)
			for i := 0; i < goroutines*times; i++ {
				timer.Reset(3 * time.Second)
				select {
				case b := <-Node.Test.BroadcastTestMsg:
					recv.Write(b)
					recv.WriteString("\n")
				case <-timer.C:
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
				need := fmt.Sprintf("test broadcast %d", i)
				require.True(t, strings.Contains(str, need), "lost: %s", need)
			}

		}(n, cNode)
	}
	wg.Wait()

	// clean
	for i := 0; i < 3; i++ {
		cNodes[i].Exit(nil)
	}
	testsuite.IsDestroyed(t, &cNodes)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)
}

// One Common Node connect the Initial Node
// Controller connect the Initial Node
// Controller send test messages
func TestCtrl_SendToNode_PassInitialNode(t *testing.T) {
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
		spew.Dump(nrr)
		err = ctrl.AcceptRegisterNode(nrr, false)
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("read CTRL.Test.NodeRegisterRequest timeout")
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
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := ctrl.Send(protocol.Node, cNodeGUID, messages.CMDBTest, msg)
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
}

// One Beacon connect the Initial Node, Controller connect the Initial Node,
// Controller send test messages to Beacon in interactive mode.
func TestCtrl_SendToBeacon_PassInitialNode(t *testing.T) {
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
		spew.Dump(brr)
		err = ctrl.AcceptRegisterBeacon(brr)
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("read CTRL.Test.BeaconRegisterRequest timeout")
	}

	timer := time.AfterFunc(10*time.Second, func() {
		t.Fatal("beacon register timeout")
	})
	Beacon.Wait()
	timer.Stop()

	// try to connect initial node
	err = Beacon.Synchronize(context.Background(), iNode.GUID(), bListener)
	require.NoError(t, err)

	// controller send messages
	BeaconGUID := Beacon.GUID()
	Beacon.Test.EnableTestMessage()
	ctrl.EnableInteractiveMode(BeaconGUID)

	const (
		goroutines = 256
		times      = 64
	)
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := ctrl.Send(protocol.Beacon, BeaconGUID, messages.CMDBTest, msg)
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
}

func TestNode_SendDirectly(t *testing.T) {
	Node := generateInitialNodeAndTrust(t)
	NodeGUID := Node.GUID()

	ctrl.Test.EnableRoleSendTestMessage()
	ch := ctrl.Test.CreateNodeSendTestMessageChannel(NodeGUID)

	const (
		goroutines = 256
		times      = 64
	)
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := Node.Send(messages.CMDBTest, msg)
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
}

// One Common Node connect the Initial Node, Controller connect the Initial Node,
// Node send test messages to Controller
func TestNode_Send_PassInitialNode(t *testing.T) {
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
		spew.Dump(nrr)
		err = ctrl.AcceptRegisterNode(nrr, false)
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("read CTRL.Test.NodeRegisterRequest timeout")
	}

	// wait Common Node
	timer := time.AfterFunc(10*time.Second, func() {
		t.Fatal("node register timeout")
	})
	cNode.Wait()
	timer.Stop()

	// try to connect initial node
	err = cNode.Synchronize(context.Background(), iNode.GUID(), bListener)
	require.NoError(t, err)

	// controller send messages
	ctrl.Test.EnableRoleSendTestMessage()
	ch := ctrl.Test.CreateNodeSendTestMessageChannel(cNode.GUID())

	const (
		goroutines = 256
		times      = 64
	)
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := cNode.Send(messages.CMDBTest, msg)
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
}

// One Beacon connect the Initial Node, Controller connect the Initial Node,
// Beacon send test messages to Controller in interactive mode.
func TestBeacon_Send_PassInitialNode(t *testing.T) {
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
		spew.Dump(brr)
		err = ctrl.AcceptRegisterBeacon(brr)
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("read CTRL.Test.BeaconRegisterRequest timeout")
	}

	timer := time.AfterFunc(10*time.Second, func() {
		t.Fatal("beacon register timeout")
	})
	Beacon.Wait()
	timer.Stop()

	// try to connect initial node
	err = Beacon.Synchronize(context.Background(), iNode.GUID(), bListener)
	require.NoError(t, err)

	// controller send messages
	ctrl.Test.EnableRoleSendTestMessage()
	ch := ctrl.Test.CreateBeaconSendTestMessageChannel(Beacon.GUID())

	const (
		goroutines = 256
		times      = 64
	)
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := Beacon.Send(messages.CMDBTest, msg)
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
}

// One Beacon connect the Common Node, Common Node Connect the Initial Node,
// Controller connect the Initial Node, Beacon send test messages to
// Controller in interactive mode.
//
// Beacon -> Common Node -> Initial Node -> Controller
func TestBeacon_Send_PassCommonNode(t *testing.T) {
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

	// Common Node synchronize with Initial Node
	err = cNode.Synchronize(context.Background(), iNode.GUID(), bListener)
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
		spew.Dump(brr)
		err = ctrl.AcceptRegisterBeacon(brr)
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("read CTRL.Test.BeaconRegisterRequest timeout")
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
	err = Beacon.Synchronize(context.Background(), cNode.GUID(), &targetListener)
	require.NoError(t, err)

	// Beacon send messages
	ctrl.Test.EnableRoleSendTestMessage()
	ch := ctrl.Test.CreateBeaconSendTestMessageChannel(Beacon.GUID())

	const (
		goroutines = 256
		times      = 64
	)
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := Beacon.Send(messages.CMDBTest, msg)
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
}

// One Beacon connect the Common Node, Common Node Connect the Initial Node,
// Controller connect the Initial Node, Beacon send test messages to
// Controller in interactive mode.
//
// Controller -> Initial Node -> Common Node -> Beacon
func TestCtrl_SendToBeacon_PassICNodes(t *testing.T) {
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

	// Common Node synchronize with Initial Node
	err = cNode.Synchronize(context.Background(), iNode.GUID(), bListener)
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
		spew.Dump(brr)
		err = ctrl.AcceptRegisterBeacon(brr)
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("read CTRL.Test.BeaconRegisterRequest timeout")
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
	err = Beacon.Synchronize(context.Background(), cNode.GUID(), &targetListener)
	require.NoError(t, err)

	// Beacon send messages
	Beacon.Test.EnableTestMessage()
	BeaconGUID := Beacon.GUID()
	ctrl.EnableInteractiveMode(BeaconGUID)

	const (
		goroutines = 256
		times      = 64
	)
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := ctrl.Send(protocol.Beacon, BeaconGUID, messages.CMDBTest, msg)
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
}
