package controller

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/xnet"
	"project/node"
)

func testGenerateNodeAndConnect(t testing.TB) *node.Node {
	testInitCtrl(t)
	NODE := testGenerateNode(t)
	listener, err := NODE.GetListener(testListenerTag)
	require.NoError(t, err)
	n := bootstrap.Node{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: listener.Addr().String(),
	}
	// trust node
	req, err := ctrl.TrustNode(context.Background(), &n)
	require.NoError(t, err)
	err = ctrl.ConfirmTrustNode(context.Background(), &n, req)
	require.NoError(t, err)
	// connect
	err = ctrl.sender.Connect(&n, NODE.GUID())
	require.NoError(t, err)
	return NODE
}

func TestSender_Connect(t *testing.T) {
	NODE := testGenerateNodeAndConnect(t)
	defer NODE.Exit(nil)
	guid := strings.ToUpper(hex.EncodeToString(NODE.GUID()))
	err := ctrl.sender.Disconnect(guid)
	require.NoError(t, err)
}

func TestSender_Broadcast(t *testing.T) {
	NODE := testGenerateNodeAndConnect(t)
	const (
		goRoutines = 256
		times      = 64
	)
	broadcast := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test broadcast %d", i))
			err := ctrl.sender.Broadcast(messages.CMDBytesTest, msg)
			if err != nil {
				t.Error(err)
				return
			}
		}
	}
	for i := 0; i < goRoutines; i++ {
		go broadcast(i * times)
	}
	recv := bytes.Buffer{}
	timer := time.NewTimer(3 * time.Second)
	for i := 0; i < goRoutines*times; i++ {
		timer.Reset(3 * time.Second)
		select {
		case b := <-NODE.Debug.Broadcast:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read NODE.Debug.Broadcast timeout i: %d", i)
		}
	}
	select {
	case <-NODE.Debug.Broadcast:
		t.Fatal("redundancy broadcast")
	case <-time.After(time.Second):
	}
	str := recv.String()
	for i := 0; i < goRoutines*times; i++ {
		need := fmt.Sprintf("test broadcast %d", i)
		require.True(t, strings.Contains(str, need), "lost: %s", need)
	}
	// clean
	guid := strings.ToUpper(hex.EncodeToString(NODE.GUID()))
	err := ctrl.sender.Disconnect(guid)
	require.NoError(t, err)
	NODE.Exit(nil)

	// testsuite.IsDestroyed(t, NODE)
}

func TestSender_SendToNode(t *testing.T) {
	NODE := testGenerateNodeAndConnect(t)
	nodeGUID := NODE.GUID()
	const (
		goRoutines = 256
		times      = 64
	)
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := ctrl.sender.Send(protocol.Node, nodeGUID, messages.CMDBytesTest, msg)
			if err != nil {
				t.Error(err)
				return
			}
		}
	}
	for i := 0; i < goRoutines; i++ {
		go send(i * times)
	}
	recv := bytes.Buffer{}
	timer := time.NewTimer(3 * time.Second)
	for i := 0; i < goRoutines*times; i++ {
		timer.Reset(3 * time.Second)
		select {
		case b := <-NODE.Debug.Send:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read NODE.Debug.Send timeout i: %d", i)
		}
	}
	select {
	case <-NODE.Debug.Send:
		t.Fatal("redundancy send")
	case <-time.After(time.Second):
	}
	str := recv.String()
	for i := 0; i < goRoutines*times; i++ {
		need := fmt.Sprintf("test send %d", i)
		require.True(t, strings.Contains(str, need), "lost: %s", need)
	}
	// clean
	guid := strings.ToUpper(hex.EncodeToString(nodeGUID))
	err := ctrl.sender.Disconnect(guid)
	require.NoError(t, err)
	NODE.Exit(nil)

	// TODO check Node
	// testsuite.IsDestroyed(t, NODE)
}

func BenchmarkSender_Broadcast(b *testing.B) {
	b.SkipNow()
	number := runtime.NumCPU()
	NODEs := make([]*node.Node, number)
	for i := 0; i < number; i++ {
		NODEs[i] = testGenerateNodeAndConnect(b)
	}
	defer func() {
		for i := 0; i < number; i++ {
			guid := strings.ToUpper(hex.EncodeToString(NODEs[i].GUID()))
			err := ctrl.sender.Disconnect(guid)
			require.NoError(b, err)
			NODEs[i].Exit(nil)
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
				case <-NODEs[index].Debug.Broadcast:
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

func TestSender_SendToNodeBenchmark(t *testing.T) {
	NODE := testGenerateNodeAndConnect(t)
	// send to Node
	nodeGUID := NODE.GUID()
	var (
		goRoutines = runtime.NumCPU()
		times      = 10000
	)
	start := time.Now()
	send := func(start int) {
		for i := start; i < start+times; i++ {
			msg := []byte(fmt.Sprintf("test send %d", i))
			err := ctrl.sender.Send(protocol.Node, nodeGUID, messages.CMDBytesTest, msg)
			if err != nil {
				t.Error(err)
				return
			}
		}
	}
	for i := 0; i < goRoutines; i++ {
		go send(i * times)
	}
	total := goRoutines * times
	timer := time.NewTimer(3 * time.Second)
	for i := 0; i < total; i++ {
		timer.Reset(3 * time.Second)
		select {
		case <-NODE.Debug.Send:
		case <-timer.C:
			t.Fatalf("read NODE.Debug.Send timeout i: %d", i)
		}
	}
	stop := time.Since(start).Seconds()
	t.Logf("[benchmark] total time: %.2fs, times: %d", stop, total)
	select {
	case <-NODE.Debug.Send:
		t.Fatal("redundancy send")
	case <-time.After(time.Second):
	}
	// clean
	guid := strings.ToUpper(hex.EncodeToString(nodeGUID))
	err := ctrl.sender.Disconnect(guid)
	require.NoError(t, err)
	NODE.Exit(nil)

	// testsuite.IsDestroyed(t, NODE)
}
