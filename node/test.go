package node

// Test contains test data
type Test struct {
	// Node.Main()
	SkipSynchronizeTime bool

	// test messages from controller
	BroadcastTestMsg chan []byte
	SendTestMsg      chan []byte
}
