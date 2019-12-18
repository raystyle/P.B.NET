package node

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

var errNoConnections = fmt.Errorf("no connections")

type forwarder struct {
	ctx *Node

	maxCtrlConns   atomic.Value
	maxNodeConns   atomic.Value
	maxBeaconConns atomic.Value

	ctrlConns      map[string]*ctrlConn
	ctrlConnsRWM   sync.RWMutex
	nodeConns      map[string]*nodeConn
	nodeConnsRWM   sync.RWMutex
	beaconConns    map[string]*beaconConn
	beaconConnsRWM sync.RWMutex

	stopSignal chan struct{}
}

func newForwarder(ctx *Node, config *Config) (*forwarder, error) {
	cfg := config.Forwarder

	f := forwarder{}

	err := f.SetMaxCtrlConns(cfg.MaxCtrlConns)
	if err != nil {
		return nil, err
	}
	err = f.SetMaxNodeConns(cfg.MaxNodeConns)
	if err != nil {
		return nil, err
	}
	err = f.SetMaxBeaconConns(cfg.MaxBeaconConns)
	if err != nil {
		return nil, err
	}

	f.ctx = ctx
	f.ctrlConns = make(map[string]*ctrlConn, cfg.MaxCtrlConns)
	f.nodeConns = make(map[string]*nodeConn, cfg.MaxNodeConns)
	f.beaconConns = make(map[string]*beaconConn, cfg.MaxBeaconConns)
	f.stopSignal = make(chan struct{})
	return &f, nil
}

func (f *forwarder) SetMaxCtrlConns(n int) error {
	if n < 1 {
		return errors.New("max controller connection must > 0")
	}
	f.maxCtrlConns.Store(n)
	return nil
}

func (f *forwarder) SetMaxNodeConns(n int) error {
	if n < 8 {
		return errors.New("max node connection must >= 8")
	}
	f.maxNodeConns.Store(n)
	return nil
}

func (f *forwarder) SetMaxBeaconConns(n int) error {
	if n < 64 {
		return errors.New("max beacon connection must >= 64")
	}
	f.maxBeaconConns.Store(n)
	return nil
}

func (f *forwarder) GetMaxCtrlConns() int {
	return f.maxCtrlConns.Load().(int)
}

func (f *forwarder) GetMaxNodeConns() int {
	return f.maxNodeConns.Load().(int)
}

func (f *forwarder) GetMaxBeaconConns() int {
	return f.maxBeaconConns.Load().(int)
}

func (f *forwarder) RegisterCtrl(tag string, conn *ctrlConn) error {
	f.ctrlConnsRWM.Lock()
	defer f.ctrlConnsRWM.Unlock()
	if len(f.ctrlConns) >= f.GetMaxCtrlConns() {
		return errors.New("max controller connections")
	}
	if _, ok := f.ctrlConns[tag]; !ok {
		f.ctrlConns[tag] = conn
	}
	return nil
}

func (f *forwarder) LogoffCtrl(tag string) {
	f.ctrlConnsRWM.Lock()
	defer f.ctrlConnsRWM.Unlock()
	if _, ok := f.ctrlConns[tag]; ok {
		delete(f.ctrlConns, tag)
	}
}

func (f *forwarder) RegisterNode(tag string, conn *nodeConn) error {
	f.nodeConnsRWM.Lock()
	defer f.nodeConnsRWM.Unlock()
	if len(f.nodeConns) >= f.GetMaxCtrlConns() {
		return errors.New("max node connections")
	}
	if _, ok := f.nodeConns[tag]; !ok {
		f.nodeConns[tag] = conn
	}
	return nil
}

func (f *forwarder) LogoffNode(tag string) {
	f.nodeConnsRWM.Lock()
	defer f.nodeConnsRWM.Unlock()
	if _, ok := f.nodeConns[tag]; ok {
		delete(f.nodeConns, tag)
	}
}

func (f *forwarder) RegisterBeacon(tag string, conn *beaconConn) error {
	f.beaconConnsRWM.Lock()
	defer f.beaconConnsRWM.Unlock()
	if len(f.beaconConns) >= f.GetMaxCtrlConns() {
		return errors.New("max beacon connections")
	}
	if _, ok := f.beaconConns[tag]; !ok {
		f.beaconConns[tag] = conn
	}
	return nil
}

func (f *forwarder) LogoffBeacon(tag string) {
	f.beaconConnsRWM.Lock()
	defer f.beaconConnsRWM.Unlock()
	if _, ok := f.beaconConns[tag]; ok {
		delete(f.beaconConns, tag)
	}
}

func (f *forwarder) GetCtrlConns() map[string]*ctrlConn {
	f.ctrlConnsRWM.RLock()
	defer f.ctrlConnsRWM.RUnlock()
	conns := make(map[string]*ctrlConn, len(f.ctrlConns))
	for tag, conn := range f.ctrlConns {
		conns[tag] = conn
	}
	return conns
}

func (f *forwarder) GetNodeConns() map[string]*nodeConn {
	f.ctrlConnsRWM.RLock()
	defer f.ctrlConnsRWM.RUnlock()
	conns := make(map[string]*nodeConn, len(f.nodeConns))
	for tag, conn := range f.nodeConns {
		conns[tag] = conn
	}
	return conns
}

func (f *forwarder) GetBeaconConns() map[string]*beaconConn {
	f.ctrlConnsRWM.RLock()
	defer f.ctrlConnsRWM.RUnlock()
	conns := make(map[string]*beaconConn, len(f.beaconConns))
	for tag, conn := range f.beaconConns {
		conns[tag] = conn
	}
	return conns
}

func (f *forwarder) log(l logger.Level, log ...interface{}) {
	f.ctx.logger.Print(l, "sender", log...)
}

type fAck interface {
	Acknowledge(guid, message []byte) (ar *protocol.AcknowledgeResponse)
}

func (f *forwarder) AckToNodeAndCtrl(guid, data []byte, except string) *protocol.AcknowledgeResult {
	ctrlConns := f.GetCtrlConns()
	nodeConns := f.GetNodeConns()
	var (
		conns map[string]fAck
		l     int
	)
	if except != "" {
		l = len(ctrlConns) + len(nodeConns) - 1
	} else {
		l = len(ctrlConns) + len(nodeConns)
	}
	if l < 1 {
		return &protocol.AcknowledgeResult{
			Err: errNoConnections,
		}
	}
	conns = make(map[string]fAck, l)
	for tag, conn := range ctrlConns {
		if tag != except {
			conns[tag] = conn
		}
	}
	for tag, conn := range nodeConns {
		if tag != except {
			conns[tag] = conn
		}
	}
	resp := make(chan *protocol.AcknowledgeResponse, l)
	for _, conn := range conns {
		go func(c fAck) {
			defer func() {
				if r := recover(); r != nil {
					err := xpanic.Error(r, "forwarder.AckToNodeAndCtrl")
					f.log(logger.Fatal, err)
				}
			}()
			resp <- c.Acknowledge(guid, data)
		}(conn)
	}
	ar := protocol.AcknowledgeResult{
		Responses: make([]*protocol.AcknowledgeResponse, l),
	}
	for i := 0; i < l; i++ {
		ar.Responses[i] = <-resp
		if ar.Responses[i].Err == nil {
			ar.Success++
		}
	}
	return &ar
}

type fSend interface {
	Send(guid, message []byte) (sr *protocol.SendResponse)
}

func (f *forwarder) SendToNodeAndCtrl(guid, data []byte, except string) *protocol.SendResult {
	ctrlConns := f.GetCtrlConns()
	nodeConns := f.GetNodeConns()
	var (
		conns map[string]fSend
		l     int
	)
	if except != "" {
		l = len(ctrlConns) + len(nodeConns) - 1
	} else {
		l = len(ctrlConns) + len(nodeConns)
	}
	if l < 1 {
		return &protocol.SendResult{
			Err: errNoConnections,
		}
	}
	conns = make(map[string]fSend, l)
	for tag, conn := range ctrlConns {
		if tag != except {
			conns[tag] = conn
		}
	}
	for tag, conn := range nodeConns {
		if tag != except {
			conns[tag] = conn
		}
	}
	resp := make(chan *protocol.SendResponse, l)
	for _, conn := range conns {
		go func(c fSend) {
			defer func() {
				if r := recover(); r != nil {
					err := xpanic.Error(r, "forwarder.SendToNodeAndCtrl")
					f.log(logger.Fatal, err)
				}
			}()
			resp <- c.Send(guid, data)
		}(conn)
	}
	sr := &protocol.SendResult{
		Responses: make([]*protocol.SendResponse, l),
	}
	for i := 0; i < l; i++ {
		sr.Responses[i] = <-resp
		if sr.Responses[i].Err == nil {
			sr.Success++
		}
	}
	return sr
}

func (f *forwarder) Close() {
	close(f.stopSignal)
	f.ctx = nil
}
