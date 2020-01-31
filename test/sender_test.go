package test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/messages"
	"project/internal/testsuite"
)

func TestNodeSendDirectly(t *testing.T) {
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
	timer := time.NewTimer(3 * time.Second)
	for i := 0; i < goroutines*times; i++ {
		timer.Reset(3 * time.Second)
		select {
		case b := <-ch:
			recv.Write(b)
			recv.WriteString("\n")
		case <-timer.C:
			t.Fatalf("read ctrl.Test.NodeSendTestMsg timeout i: %d", i)
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
	guid := strings.ToUpper(hex.EncodeToString(NodeGUID))
	err := ctrl.Disconnect(guid)
	require.NoError(t, err)
	Node.Exit(nil)
	testsuite.IsDestroyed(t, Node)
}
