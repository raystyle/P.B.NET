package controller

import (
	"context"
	"encoding/hex"
	"sync"

	"github.com/pkg/errors"

	"project/internal/messages"
)

// Test contains all test data
type Test struct {
	// about CTRL.Main()
	SkipTestClientDNS   bool
	SkipSynchronizeTime bool

	// about sender send test message
	roleSendTestMsgEnabled bool
	// Node send test message, key = Node GUID hex
	nodeSendTestMsg    map[string]chan []byte
	nodeSendTestMsgRWM sync.RWMutex
	// Beacon send test message , key = Beacon GUID hex
	beaconSendTestMsg    map[string]chan []byte
	beaconSendTestMsgRWM sync.RWMutex

	// about role register request
	NodeRegisterRequest   chan *messages.NodeRegisterRequest
	BeaconRegisterRequest chan *messages.BeaconRegisterRequest
}

// EnableRoleSendTestMessage is used to enable role send test message
func (t *Test) EnableRoleSendTestMessage() {
	t.roleSendTestMsgEnabled = true
}

// CreateNodeSendTestMessageChannel is used to create node send test message channel
func (t *Test) CreateNodeSendTestMessageChannel(guid []byte) chan []byte {
	key := hex.EncodeToString(guid)
	t.nodeSendTestMsgRWM.Lock()
	defer t.nodeSendTestMsgRWM.Unlock()
	if t.nodeSendTestMsg == nil {
		t.nodeSendTestMsg = make(map[string]chan []byte)
	}
	if ch, ok := t.nodeSendTestMsg[key]; ok {
		return ch
	}
	ch := make(chan []byte, 4)
	t.nodeSendTestMsg[key] = ch
	return ch
}

// CreateBeaconSendTestMessageChannel is used to create beacon send test message channel
func (t *Test) CreateBeaconSendTestMessageChannel(guid []byte) chan []byte {
	key := hex.EncodeToString(guid)
	t.beaconSendTestMsgRWM.Lock()
	defer t.beaconSendTestMsgRWM.Unlock()
	if t.beaconSendTestMsg == nil {
		t.beaconSendTestMsg = make(map[string]chan []byte)
	}
	if ch, ok := t.beaconSendTestMsg[key]; ok {
		return ch
	}
	ch := make(chan []byte, 4)
	t.beaconSendTestMsg[key] = ch
	return ch
}

// AddNodeSendTestMessage is used to add node send test message
func (t *Test) AddNodeSendTestMessage(ctx context.Context, guid, message []byte) error {
	key := hex.EncodeToString(guid)
	t.nodeSendTestMsgRWM.Lock()
	defer t.nodeSendTestMsgRWM.Unlock()
	ch, ok := t.nodeSendTestMsg[key]
	if !ok {
		return errors.Errorf("node: %X doesn't exists", guid)
	}
	select {
	case ch <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AddBeaconSendTestMessage is used to add beacon send test message
func (t *Test) AddBeaconSendTestMessage(ctx context.Context, guid, message []byte) error {
	key := hex.EncodeToString(guid)
	t.beaconSendTestMsgRWM.Lock()
	defer t.beaconSendTestMsgRWM.Unlock()
	ch, ok := t.beaconSendTestMsg[key]
	if !ok {
		return errors.Errorf("beacon: %X doesn't exists", guid)
	}
	select {
	case ch <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CreateNodeRegisterRequestChannel is used to create node register request channel
func (t *Test) CreateNodeRegisterRequestChannel() {
	if t.NodeRegisterRequest == nil {
		t.NodeRegisterRequest = make(chan *messages.NodeRegisterRequest, 4)
	}
}

// CreateBeaconRegisterRequestChannel is used to create beacon register request channel
func (t *Test) CreateBeaconRegisterRequestChannel() {
	if t.BeaconRegisterRequest == nil {
		t.BeaconRegisterRequest = make(chan *messages.BeaconRegisterRequest, 4)
	}
}
