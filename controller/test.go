package controller

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"project/internal/guid"
	"project/internal/messages"
)

// Test contains all test data
type Test struct {
	// about Ctrl.Main()
	SkipTestClientDNS   bool
	SkipSynchronizeTime bool

	// about sender send test message
	roleSendTestMsgEnabled bool
	// Node send test message, key = Node GUID hex
	nodeSendTestMsg    map[guid.GUID]chan []byte
	nodeSendTestMsgRWM sync.RWMutex
	// Beacon send test message , key = Beacon GUID hex
	beaconSendTestMsg    map[guid.GUID]chan []byte
	beaconSendTestMsgRWM sync.RWMutex

	// about role register request
	NodeRegisterRequest   chan *messages.NodeRegisterRequest
	BeaconRegisterRequest chan *messages.BeaconRegisterRequest
}

// EnableRoleSendTestMessage is used to enable role send test message
func (t *Test) EnableRoleSendTestMessage() {
	if !t.roleSendTestMsgEnabled {
		t.roleSendTestMsgEnabled = true
		t.nodeSendTestMsg = make(map[guid.GUID]chan []byte)
		t.beaconSendTestMsg = make(map[guid.GUID]chan []byte)
	}
}

// CreateNodeSendTestMessageChannel is used to create node send test message channel
func (t *Test) CreateNodeSendTestMessageChannel(guid *guid.GUID) chan []byte {
	t.nodeSendTestMsgRWM.Lock()
	defer t.nodeSendTestMsgRWM.Unlock()
	if ch, ok := t.nodeSendTestMsg[*guid]; ok {
		return ch
	}
	ch := make(chan []byte, 4)
	t.nodeSendTestMsg[*guid] = ch
	return ch
}

// CreateBeaconSendTestMessageChannel is used to create beacon send test message channel
func (t *Test) CreateBeaconSendTestMessageChannel(guid *guid.GUID) chan []byte {
	t.beaconSendTestMsgRWM.Lock()
	defer t.beaconSendTestMsgRWM.Unlock()
	if ch, ok := t.beaconSendTestMsg[*guid]; ok {
		return ch
	}
	ch := make(chan []byte, 4)
	t.beaconSendTestMsg[*guid] = ch
	return ch
}

// AddNodeSendTestMessage is used to add node send test message
func (t *Test) AddNodeSendTestMessage(ctx context.Context, guid *guid.GUID, message []byte) error {
	t.nodeSendTestMsgRWM.Lock()
	defer t.nodeSendTestMsgRWM.Unlock()
	ch, ok := t.nodeSendTestMsg[*guid]
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
func (t *Test) AddBeaconSendTestMessage(ctx context.Context, guid *guid.GUID, message []byte) error {
	t.beaconSendTestMsgRWM.Lock()
	defer t.beaconSendTestMsgRWM.Unlock()
	ch, ok := t.beaconSendTestMsg[*guid]
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
