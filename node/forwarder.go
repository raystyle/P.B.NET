package node

import (
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
)

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
}

func newForwarder(ctx *Node, config *Config) (*forwarder, error) {
	cfg := config.Forwarder

	f := forwarder{
		ctx: ctx,
	}

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

	f.ctrlConns = make(map[string]*ctrlConn, cfg.MaxCtrlConns)
	f.nodeConns = make(map[string]*nodeConn, cfg.MaxNodeConns)
	f.beaconConns = make(map[string]*beaconConn, cfg.MaxBeaconConns)
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

func (f *forwarder) Close() {
	f.ctx = nil
}
