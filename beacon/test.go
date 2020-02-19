package beacon

import (
	"context"
	"sync"
)

// Test contains all test data
type Test struct {
	// Beacon.Main()
	SkipSynchronizeTime bool

	// about sender send test message
	testMsgEnabled    bool
	testMsgEnabledRWM sync.RWMutex

	// test messages from controller
	SendTestMsg chan []byte
}

// EnableTestMessage is used to enable Controller send test message.
func (t *Test) EnableTestMessage() {
	t.testMsgEnabledRWM.Lock()
	defer t.testMsgEnabledRWM.Unlock()
	if !t.testMsgEnabled {
		t.SendTestMsg = make(chan []byte, 4)
		t.testMsgEnabled = true
	}
}

// AddSendTestMessage is used to add Controller send test message.
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
