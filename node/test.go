package node

import (
	"context"
	"sync"
)

// Test contains all test data.
type Test struct {
	options struct {
		SkipSynchronizeTime bool
	}

	// about sender send test message
	testMsgEnabled    bool
	testMsgEnabledRWM sync.RWMutex

	// test messages from controller
	BroadcastTestMsg chan []byte
	SendTestMsg      chan []byte
}

// EnableTestMessage is used to enable controller send test message
func (t *Test) EnableTestMessage() {
	t.testMsgEnabledRWM.Lock()
	defer t.testMsgEnabledRWM.Unlock()
	if !t.testMsgEnabled {
		t.BroadcastTestMsg = make(chan []byte, 4)
		t.SendTestMsg = make(chan []byte, 4)
		t.testMsgEnabled = true
	}
}

// AddBroadcastTestMessage is used to add controller broadcast test message
func (t *Test) AddBroadcastTestMessage(ctx context.Context, message []byte) error {
	t.testMsgEnabledRWM.RLock()
	defer t.testMsgEnabledRWM.RUnlock()
	if !t.testMsgEnabled {
		return nil
	}
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
	t.testMsgEnabledRWM.RLock()
	defer t.testMsgEnabledRWM.RUnlock()
	if !t.testMsgEnabled {
		return nil
	}
	msg := make([]byte, len(message))
	copy(msg, message)
	select {
	case t.SendTestMsg <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
