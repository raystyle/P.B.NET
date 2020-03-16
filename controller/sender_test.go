package controller

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/messages"
	"project/internal/testsuite"

	"project/node"
)

func testGenerateInitialNodeAndTrust(t testing.TB) *node.Node {
	Node := testGenerateInitialNode(t)

	listener := testGetNodeListener(t, Node, testInitialNodeListenerTag)
	// trust node
	nnr, err := ctrl.TrustNode(context.Background(), listener)
	require.NoError(t, err)
	// confirm
	reply := ReplyNodeRegister{
		ID:        nnr.ID,
		Bootstrap: true,
		Zone:      "test",
	}
	err = ctrl.ConfirmTrustNode(context.Background(), &reply)
	require.NoError(t, err)
	// connect
	err = ctrl.Synchronize(context.Background(), Node.GUID(), listener)
	require.NoError(t, err)
	return Node
}

func TestSender_Connect(t *testing.T) {
	Node := testGenerateInitialNodeAndTrust(t)
	nodeGUID := Node.GUID()

	err := ctrl.Disconnect(nodeGUID)
	require.NoError(t, err)

	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)

	err = ctrl.DeleteNodeUnscoped(nodeGUID)
	require.NoError(t, err)
}

func TestSender_Broadcast(t *testing.T) {
	Node := testGenerateInitialNodeAndTrust(t)
	nodeGUID := Node.GUID()
	Node.Test.EnableMessage()

	// broadcast
	const (
		goroutines = 256
		times      = 64
	)
	broadcast := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test broadcast %d", i))
			err := ctrl.sender.Broadcast(messages.CMDBTest, msg, true)
			require.NoError(t, err)
		}
	}
	for i := 0; i < goroutines; i++ {
		go broadcast(i * times)
	}
	recv := make(map[string]struct{}, goroutines*times)
	timer := time.NewTimer(3 * time.Second)
	for i := 0; i < goroutines*times; i++ {
		timer.Reset(3 * time.Second)
		select {
		case msg := <-Node.Test.BroadcastMsg:
			recv[string(msg)] = struct{}{}
		case <-timer.C:
			t.Fatalf("read NODE.Test.BroadcastMsg timeout i: %d", i)
		}
	}
	select {
	case <-Node.Test.BroadcastMsg:
		t.Fatal("redundancy broadcast")
	case <-time.After(time.Second):
	}
	for i := 0; i < goroutines*times; i++ {
		need := fmt.Sprintf("test broadcast %d", i)
		_, ok := recv[need]
		require.True(t, ok, "lost: %s", need)
	}

	// clean
	err := ctrl.sender.Disconnect(nodeGUID)
	require.NoError(t, err)

	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)

	err = ctrl.DeleteNodeUnscoped(nodeGUID)
	require.NoError(t, err)
}

func TestSender_SendToNode(t *testing.T) {
	Node := testGenerateInitialNodeAndTrust(t)
	nodeGUID := Node.GUID()
	Node.Test.EnableMessage()

	// send
	const (
		goroutines = 256
		times      = 64
	)
	ctx := context.Background()
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := ctrl.sender.SendToNode(ctx, nodeGUID, messages.CMDBTest, msg, true)
			require.NoError(t, err)
		}
	}
	for i := 0; i < goroutines; i++ {
		go send(i * times)
	}
	// read
	recv := make(map[string]struct{}, goroutines*times)
	timer := time.NewTimer(3 * time.Second)
	for i := 0; i < goroutines*times; i++ {
		timer.Reset(3 * time.Second)
		select {
		case msg := <-Node.Test.SendMsg:
			recv[string(msg)] = struct{}{}
		case <-timer.C:
			t.Fatalf("read Node.Test.SendMsg timeout i: %d", i)
		}
	}
	select {
	case <-Node.Test.SendMsg:
		t.Fatal("redundancy send")
	case <-time.After(time.Second):
	}
	for i := 0; i < goroutines*times; i++ {
		need := fmt.Sprintf("test send %d", i)
		_, ok := recv[need]
		require.True(t, ok, "lost: %s", need)
	}

	// clean
	err := ctrl.sender.Disconnect(nodeGUID)
	require.NoError(t, err)

	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)

	err = ctrl.DeleteNodeUnscoped(nodeGUID)
	require.NoError(t, err)
}

func BenchmarkSender_Broadcast(b *testing.B) {
	b.Skip()
	number := runtime.NumCPU()
	Nodes := make([]*node.Node, number)
	for i := 0; i < number; i++ {
		Nodes[i] = testGenerateInitialNodeAndTrust(b)
	}
	defer func() {
		for i := 0; i < number; i++ {
			err := ctrl.sender.Disconnect(Nodes[i].GUID())
			require.NoError(b, err)
			Nodes[i].Exit(nil)
		}
	}()
	count := 0
	countM := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(number)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < number; i++ {
		go func(index int) {
			defer wg.Done()
			timer := time.NewTimer(3 * time.Second)
			for {
				timer.Reset(3 * time.Second)
				select {
				case <-Nodes[index].Test.BroadcastMsg:
					countM.Lock()
					count++
					countM.Unlock()
				case <-timer.C:
					return
				}
			}
		}(i)
	}
	wg.Wait()
	b.StopTimer()
}

func TestBenchmarkSender_SendToNode(t *testing.T) {
	Node := testGenerateInitialNodeAndTrust(t)
	nodeGUID := Node.GUID()
	Node.Test.EnableMessage()

	var (
		goroutines = runtime.NumCPU()
		times      = 60000
	)
	ctx := context.Background()
	start := time.Now()
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := ctrl.sender.SendToNode(ctx, nodeGUID, messages.CMDBTest, msg, false)
			require.NoError(t, err)
		}
	}
	for i := 0; i < goroutines; i++ {
		go send(i * times)
	}
	total := goroutines * times
	timer := time.NewTimer(3 * time.Second)
	for i := 0; i < total; i++ {
		timer.Reset(3 * time.Second)
		select {
		case <-Node.Test.SendMsg:
		case <-timer.C:
			t.Fatalf("read Node.Test.SendMsg timeout i: %d", i)
		}
	}
	stop := time.Since(start).Seconds()
	t.Logf("[benchmark] total time: %.2fs, times: %d", stop, total)
	select {
	case <-Node.Test.SendMsg:
		t.Fatal("redundancy send")
	case <-time.After(time.Second):
	}

	// clean
	err := ctrl.sender.Disconnect(nodeGUID)
	require.NoError(t, err)

	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)

	err = ctrl.DeleteNodeUnscoped(nodeGUID)
	require.NoError(t, err)
}
