package node

import (
	"context"
)

// Test contains all test data
type Test struct {
	// Node.Main()
	SkipSynchronizeTime bool

	// about sender send test message
	testMsgEnabled bool
	// test messages from controller
	BroadcastTestMsg chan []byte
	SendTestMsg      chan []byte
}

// EnableTestMessage is used to enable controller send test message
func (t *Test) EnableTestMessage() {
	t.testMsgEnabled = true
	t.BroadcastTestMsg = make(chan []byte, 4)
	t.SendTestMsg = make(chan []byte, 4)
}

// AddBroadcastTestMessage is used to add controller broadcast test message
func (t *Test) AddBroadcastTestMessage(ctx context.Context, message []byte) error {
	msg := make([]byte, len(message))
	copy(msg, message)
	select {
	case t.BroadcastTestMsg <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AddSendTestMessage is used to add controller send test message
func (t *Test) AddSendTestMessage(ctx context.Context, message []byte) error {
	msg := make([]byte, len(message))
	copy(msg, message)
	select {
	case t.SendTestMsg <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
