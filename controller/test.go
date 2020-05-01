package controller

import (
	"context"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

	"project/internal/guid"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/xpanic"
)

// Test contains all test data.
type Test struct {
	ctx *Ctrl

	options struct {
		SkipTestClientDNS   bool
		SkipSynchronizeTime bool
	}

	// about role register request
	nodeListeners          map[guid.GUID][]string
	nodeListenersRWM       sync.RWMutex
	NoticeNodeRegister     chan *NoticeNodeRegister
	noticeNodeRegisterMu   sync.Mutex
	NoticeBeaconRegister   chan *NoticeBeaconRegister
	noticeBeaconRegisterMu sync.Mutex

	// about sender send test message
	roleSendMsgEnabled    bool
	roleSendMsgEnabledRWM sync.RWMutex

	// Node and Beacon send test message
	nodeSendMsg      map[guid.GUID]chan []byte
	nodeSendMsgRWM   sync.RWMutex
	beaconSendMsg    map[guid.GUID]chan []byte
	beaconSendMsgRWM sync.RWMutex

	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newTest(ctx *Ctrl, config *Config) *Test {
	test := Test{
		ctx:           ctx,
		options:       config.Test,
		nodeListeners: make(map[guid.GUID][]string),
	}
	test.context, test.cancel = context.WithCancel(context.Background())
	return &test
}

func (t *Test) log(lv logger.Level, log ...interface{}) {
	t.ctx.logger.Println(lv, "test", log...)
}

// EnableRegisterNode is used to create notice Node register channel.
func (t *Test) EnableRegisterNode() bool {
	t.noticeNodeRegisterMu.Lock()
	defer t.noticeNodeRegisterMu.Unlock()
	if t.NoticeNodeRegister != nil {
		return false
	}
	t.NoticeNodeRegister = make(chan *NoticeNodeRegister, 4)
	return true
}

// EnableRegisterBeacon is used to create notice Beacon register channel.
func (t *Test) EnableRegisterBeacon() bool {
	t.noticeBeaconRegisterMu.Lock()
	defer t.noticeBeaconRegisterMu.Unlock()
	if t.NoticeBeaconRegister != nil {
		return false
	}
	t.NoticeBeaconRegister = make(chan *NoticeBeaconRegister, 4)
	return true
}

// SetNodeListener is used to set Node listener that will configure to ReplyRoleRegister.
func (t *Test) SetNodeListener(listeners map[guid.GUID][]string) {
	t.nodeListenersRWM.Lock()
	defer t.nodeListenersRWM.Unlock()
	t.nodeListeners = listeners
}

func (t *Test) getNodeListener() map[guid.GUID][]string {
	t.nodeListenersRWM.RLock()
	defer t.nodeListenersRWM.RUnlock()
	return t.nodeListeners
}

// EnableAutoRegisterNode is used to accept Node register request automatically.
func (t *Test) EnableAutoRegisterNode() {
	if !t.EnableRegisterNode() {
		return
	}
	t.wg.Add(1)
	go t.registerNode()
}

func (t *Test) registerNode() {
	defer func() {
		if r := recover(); r != nil {
			t.log(logger.Fatal, xpanic.Print(r, "Test.registerNode"))
			// restart
			time.Sleep(time.Second)
			go t.registerNode()
		} else {
			t.wg.Done()
		}
	}()
	var (
		nnr *NoticeNodeRegister
		err error
	)
	for {
		select {
		case nnr = <-t.NoticeNodeRegister:
			spew.Dump(nnr)
			reply := ReplyNodeRegister{
				ID:        nnr.ID,
				Result:    messages.RegisterResultAccept,
				Zone:      "test",
				Listeners: t.getNodeListener(),
			}
			err = t.ctx.ReplyNodeRegister(t.context, &reply)
			if err != nil {
				t.log(logger.Error, "failed to register node:", err)
			}
		case <-t.context.Done():
			return
		}
	}
}

// EnableAutoRegisterBeacon is used to accept Beacon register request automatically.
func (t *Test) EnableAutoRegisterBeacon() {
	if !t.EnableRegisterBeacon() {
		return
	}
	t.wg.Add(1)
	go t.registerBeacon()
}

func (t *Test) registerBeacon() {
	defer func() {
		if r := recover(); r != nil {
			t.log(logger.Fatal, xpanic.Print(r, "Test.registerBeacon"))
			// restart
			time.Sleep(time.Second)
			go t.registerBeacon()
		} else {
			t.wg.Done()
		}
	}()
	var (
		nbr *NoticeBeaconRegister
		err error
	)
	for {
		select {
		case nbr = <-t.NoticeBeaconRegister:
			spew.Dump(nbr)
			reply := ReplyBeaconRegister{
				ID:        nbr.ID,
				Result:    messages.RegisterResultAccept,
				Listeners: t.getNodeListener(),
			}
			err = t.ctx.ReplyBeaconRegister(t.context, &reply)
			if err != nil {
				t.log(logger.Error, "failed to register beacon:", err)
			}
		case <-t.context.Done():
			return
		}
	}
}

// AddNoticeNodeRegister is used to add notice Node register.
func (t *Test) AddNoticeNodeRegister(ctx context.Context, nnr *NoticeNodeRegister) {
	t.noticeNodeRegisterMu.Lock()
	defer t.noticeNodeRegisterMu.Unlock()
	if t.NoticeNodeRegister == nil {
		return
	}
	select {
	case t.NoticeNodeRegister <- nnr:
	case <-ctx.Done():
	case <-t.context.Done():
	}
}

// AddNoticeBeaconRegister is used to add notice Beacon register.
func (t *Test) AddNoticeBeaconRegister(ctx context.Context, nbr *NoticeBeaconRegister) {
	t.noticeBeaconRegisterMu.Lock()
	defer t.noticeBeaconRegisterMu.Unlock()
	if t.NoticeBeaconRegister == nil {
		return
	}
	select {
	case t.NoticeBeaconRegister <- nbr:
	case <-ctx.Done():
	case <-t.context.Done():
	}
}

// EnableRoleSendMessage is used to enable role send test message.
func (t *Test) EnableRoleSendMessage() {
	t.roleSendMsgEnabledRWM.Lock()
	defer t.roleSendMsgEnabledRWM.Unlock()
	if !t.roleSendMsgEnabled {
		t.nodeSendMsg = make(map[guid.GUID]chan []byte)
		t.beaconSendMsg = make(map[guid.GUID]chan []byte)
		t.roleSendMsgEnabled = true
	}
}

// CreateNodeSendMessageChannel is used to create Node send test message channel.
func (t *Test) CreateNodeSendMessageChannel(guid *guid.GUID) chan []byte {
	t.nodeSendMsgRWM.Lock()
	defer t.nodeSendMsgRWM.Unlock()
	if ch, ok := t.nodeSendMsg[*guid]; ok {
		return ch
	}
	ch := make(chan []byte, 4)
	t.nodeSendMsg[*guid] = ch
	return ch
}

// CreateBeaconSendMessageChannel is used to create Beacon send test message channel.
func (t *Test) CreateBeaconSendMessageChannel(guid *guid.GUID) chan []byte {
	t.beaconSendMsgRWM.Lock()
	defer t.beaconSendMsgRWM.Unlock()
	if ch, ok := t.beaconSendMsg[*guid]; ok {
		return ch
	}
	ch := make(chan []byte, 4)
	t.beaconSendMsg[*guid] = ch
	return ch
}

// AddNodeSendMessage is used to add Node send test message.
func (t *Test) AddNodeSendMessage(ctx context.Context, guid *guid.GUID, message []byte) error {
	t.roleSendMsgEnabledRWM.RLock()
	defer t.roleSendMsgEnabledRWM.RUnlock()
	if !t.roleSendMsgEnabled {
		return nil
	}
	t.nodeSendMsgRWM.Lock()
	defer t.nodeSendMsgRWM.Unlock()
	ch, ok := t.nodeSendMsg[*guid]
	if !ok {
		return errors.Errorf("node: %s doesn't exist", guid.Hex())
	}
	msg := make([]byte, len(message))
	copy(msg, message)
	select {
	case ch <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-t.context.Done():
		return t.context.Err()
	}
}

// AddBeaconSendMessage is used to add Beacon send test message.
func (t *Test) AddBeaconSendMessage(ctx context.Context, guid *guid.GUID, message []byte) error {
	t.roleSendMsgEnabledRWM.RLock()
	defer t.roleSendMsgEnabledRWM.RUnlock()
	if !t.roleSendMsgEnabled {
		return nil
	}
	t.beaconSendMsgRWM.Lock()
	defer t.beaconSendMsgRWM.Unlock()
	ch, ok := t.beaconSendMsg[*guid]
	if !ok {
		return errors.Errorf("beacon: %s doesn't exist", guid.Hex())
	}
	msg := make([]byte, len(message))
	copy(msg, message)
	select {
	case ch <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-t.context.Done():
		return t.context.Err()
	}
}

// Close is used to close test module.
func (t *Test) Close() {
	t.cancel()
	t.wg.Wait()
	t.ctx = nil
}
