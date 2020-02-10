package beacon

import (
	"context"
)

// Test contains all test data
type Test struct {
	// Beacon.Main()
	SkipSynchronizeTime bool

	// about sender send test message
	testMsgEnabled bool
	// test messages from controller
	SendTestMsg chan []byte
}

// EnableTestMessage is used to enable Controller send test message.
func (t *Test) EnableTestMessage() {
	t.testMsgEnabled = true
	t.SendTestMsg = make(chan []byte, 4)
}

// AddSendTestMessage is used to add Controller send test message.
func (t *Test) AddSendTestMessage(ctx context.Context, message []byte) error {
	select {
	case t.SendTestMsg <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
