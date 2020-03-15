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
	sendMsgEnabled    bool
	sendMsgEnabledRWM sync.RWMutex

	// test messages from controller
	BroadcastMsg chan []byte
	SendMsg      chan []byte
}

// EnableMessage is used to enable controller send test message.
func (t *Test) EnableMessage() {
	t.sendMsgEnabledRWM.Lock()
	defer t.sendMsgEnabledRWM.Unlock()
	if !t.sendMsgEnabled {
		t.BroadcastMsg = make(chan []byte, 4)
		t.SendMsg = make(chan []byte, 4)
		t.sendMsgEnabled = true
	}
}

// AddBroadcastMessage is used to add controller broadcast test message.
func (t *Test) AddBroadcastMessage(ctx context.Context, message []byte) error {
	t.sendMsgEnabledRWM.RLock()
	defer t.sendMsgEnabledRWM.RUnlock()
	if !t.sendMsgEnabled {
		return nil
	}
	msg := make([]byte, len(message))
	copy(msg, message)
	select {
	case t.BroadcastMsg <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AddSendMessage is used to add controller send test message.
func (t *Test) AddSendMessage(ctx context.Context, message []byte) error {
	t.sendMsgEnabledRWM.RLock()
	defer t.sendMsgEnabledRWM.RUnlock()
	if !t.sendMsgEnabled {
		return nil
	}
	msg := make([]byte, len(message))
	copy(msg, message)
	select {
	case t.SendMsg <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
