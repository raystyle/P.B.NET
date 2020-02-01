package controller

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/testsuite"

	"project/node"
)

func TestSender_Connect(t *testing.T) {
	Node := testGenerateInitialNodeAndTrust(t)
	guid := strings.ToUpper(hex.EncodeToString(Node.GUID()))
	err := ctrl.Disconnect(guid)
	require.NoError(t, err)

	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)
}

func TestSender_SendToNode(t *testing.T) {
	Node := testGenerateInitialNodeAndTrust(t)
	nodeGUID := Node.GUID()

	const (
		goroutines = 256
		times      = 64
	)
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := ctrl.sender.Send(protocol.Node, nodeGUID, messages.CMDBTest, msg)
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
	timer := time.NewTimer(3 * time.Second)
	for i := 0; i < goroutines*times; i++ {
		timer.Reset(3 * time.Second)
		select {
		case b := <-Node.Test.SendTestMsg:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read Node.Test.Send timeout i: %d", i)
		}
	}
	select {
	case <-Node.Test.SendTestMsg:
		t.Fatal("redundancy send")
	case <-time.After(time.Second):
	}
	str := recv.String()
	for i := 0; i < goroutines*times; i++ {
		need := fmt.Sprintf("test send %d", i)
		require.True(t, strings.Contains(str, need), "lost: %s", need)
	}

	// clean
	guid := strings.ToUpper(hex.EncodeToString(nodeGUID))
	err := ctrl.sender.Disconnect(guid)
	require.NoError(t, err)
	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)
}

func TestSender_Broadcast(t *testing.T) {
	Node := testGenerateInitialNodeAndTrust(t)

	const (
		goroutines = 256
		times      = 64
	)
	broadcast := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test broadcast %d", i))
			err := ctrl.sender.Broadcast(messages.CMDBTest, msg)
			if err != nil {
				t.Error(err)
				return
			}
		}
	}
	for i := 0; i < goroutines; i++ {
		go broadcast(i * times)
	}
	recv := bytes.Buffer{}
	timer := time.NewTimer(3 * time.Second)
	for i := 0; i < goroutines*times; i++ {
		timer.Reset(3 * time.Second)
		select {
		case b := <-Node.Test.BroadcastTestMsg:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read NODE.Test.Broadcast timeout i: %d", i)
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

	// clean
	guid := strings.ToUpper(hex.EncodeToString(Node.GUID()))
	err := ctrl.sender.Disconnect(guid)
	require.NoError(t, err)
	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)
}

func TestBenchmarkSender_SendToNode(t *testing.T) {
	Node := testGenerateInitialNodeAndTrust(t)
	nodeGUID := Node.GUID()

	var (
		goroutines = runtime.NumCPU()
		times      = 600000
	)
	start := time.Now()
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := ctrl.sender.Send(protocol.Node, nodeGUID, messages.CMDBTest, msg)
			if err != nil {
				t.Error(err)
				return
			}
			// time.Sleep(time.Second)
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
		case <-Node.Test.SendTestMsg:
		case <-timer.C:
			t.Fatalf("read Node.Test.SendTestMsg timeout i: %d", i)
		}
	}
	stop := time.Since(start).Seconds()
	t.Logf("[benchmark] total time: %.2fs, times: %d", stop, total)
	select {
	case <-Node.Test.SendTestMsg:
		t.Fatal("redundancy send")
	case <-time.After(time.Second):
	}

	// clean
	guid := strings.ToUpper(hex.EncodeToString(nodeGUID))
	err := ctrl.sender.Disconnect(guid)
	require.NoError(t, err)
	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)
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
			guid := strings.ToUpper(hex.EncodeToString(Nodes[i].GUID()))
			err := ctrl.sender.Disconnect(guid)
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
				case <-Nodes[index].Test.BroadcastTestMsg:
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
