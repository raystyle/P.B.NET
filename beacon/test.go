package beacon

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
	msgEnabled    bool
	msgEnabledRWM sync.RWMutex

	// test messages from controller
	SendMsg chan []byte
}

// EnableMessage is used to enable Controller send test message.
func (t *Test) EnableMessage() {
	t.msgEnabledRWM.Lock()
	defer t.msgEnabledRWM.Unlock()
	if !t.msgEnabled {
		t.SendMsg = make(chan []byte, 4)
		t.msgEnabled = true
	}
}

// AddSendMessage is used to add Controller send test message.
func (t *Test) AddSendMessage(ctx context.Context, message []byte) error {
	t.msgEnabledRWM.RLock()
	defer t.msgEnabledRWM.RUnlock()
	if !t.msgEnabled {
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
