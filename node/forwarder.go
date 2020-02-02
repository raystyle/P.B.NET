package node

import (
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type forwarder struct {
	ctx *Node

	maxClientConns atomic.Value
	maxCtrlConns   atomic.Value
	maxNodeConns   atomic.Value
	maxBeaconConns atomic.Value

	clientConns    map[string]*Client
	clientConnsRWM sync.RWMutex
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

	err := f.SetMaxClientConns(cfg.MaxClientConns)
	if err != nil {
		return nil, err
	}
	err = f.SetMaxCtrlConns(cfg.MaxCtrlConns)
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
	f.clientConns = make(map[string]*Client, cfg.MaxClientConns)
	f.ctrlConns = make(map[string]*ctrlConn, cfg.MaxCtrlConns)
	f.nodeConns = make(map[string]*nodeConn, cfg.MaxNodeConns)
	f.beaconConns = make(map[string]*beaconConn, cfg.MaxBeaconConns)
	f.stopSignal = make(chan struct{})
	return &f, nil
}

// ---------------------------------------------client---------------------------------------------

func (f *forwarder) SetMaxClientConns(n int) error {
	if n < 1 {
		return errors.New("max client connection must > 0")
	}
	f.maxClientConns.Store(n)
	return nil
}

func (f *forwarder) GetMaxClientConns() int {
	return f.maxClientConns.Load().(int)
}

func (f *forwarder) RegisterClient(tag string, client *Client) error {
	f.clientConnsRWM.Lock()
	defer f.clientConnsRWM.Unlock()
	if len(f.clientConns) >= f.GetMaxClientConns() {
		return errors.New("max client connections")
	}
	if _, ok := f.clientConns[tag]; ok {
		return errors.Errorf("client has been register\ntag: %s", tag)
	}
	f.clientConns[tag] = client
	return nil
}

func (f *forwarder) LogoffClient(tag string) {
	f.clientConnsRWM.Lock()
	defer f.clientConnsRWM.Unlock()
	if _, ok := f.clientConns[tag]; ok {
		delete(f.clientConns, tag)
	}
}

func (f *forwarder) GetClientConns() map[string]*Client {
	f.clientConnsRWM.RLock()
	defer f.clientConnsRWM.RUnlock()
	clients := make(map[string]*Client, len(f.clientConns))
	for tag, client := range f.clientConns {
		clients[tag] = client
	}
	return clients
}

// ------------------------------------------controller--------------------------------------------

func (f *forwarder) SetMaxCtrlConns(n int) error {
	if n < 1 {
		return errors.New("max controller connection must > 0")
	}
	f.maxCtrlConns.Store(n)
	return nil
}

func (f *forwarder) GetMaxCtrlConns() int {
	return f.maxCtrlConns.Load().(int)
}

func (f *forwarder) RegisterCtrl(tag string, conn *ctrlConn) error {
	f.ctrlConnsRWM.Lock()
	defer f.ctrlConnsRWM.Unlock()
	if len(f.ctrlConns) >= f.GetMaxCtrlConns() {
		return errors.New("max controller connections")
	}
	if _, ok := f.ctrlConns[tag]; ok {
		return errors.Errorf("controller has been register\ntag: %s", tag)
	}
	f.ctrlConns[tag] = conn
	return nil
}

func (f *forwarder) LogoffCtrl(tag string) {
	f.ctrlConnsRWM.Lock()
	defer f.ctrlConnsRWM.Unlock()
	if _, ok := f.ctrlConns[tag]; ok {
		delete(f.ctrlConns, tag)
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

// ---------------------------------------------node-----------------------------------------------

func (f *forwarder) SetMaxNodeConns(n int) error {
	if n < 8 {
		return errors.New("max node connection must >= 8")
	}
	f.maxNodeConns.Store(n)
	return nil
}

func (f *forwarder) GetMaxNodeConns() int {
	return f.maxNodeConns.Load().(int)
}

func (f *forwarder) RegisterNode(tag string, conn *nodeConn) error {
	f.nodeConnsRWM.Lock()
	defer f.nodeConnsRWM.Unlock()
	if len(f.nodeConns) >= f.GetMaxNodeConns() {
		return errors.New("max node connections")
	}
	if _, ok := f.nodeConns[tag]; ok {
		return errors.Errorf("node has been register\ntag: %s", tag)
	}
	f.nodeConns[tag] = conn
	return nil
}

func (f *forwarder) LogoffNode(tag string) {
	f.nodeConnsRWM.Lock()
	defer f.nodeConnsRWM.Unlock()
	if _, ok := f.nodeConns[tag]; ok {
		delete(f.nodeConns, tag)
	}
}

func (f *forwarder) GetNodeConns() map[string]*nodeConn {
	f.nodeConnsRWM.RLock()
	defer f.nodeConnsRWM.RUnlock()
	conns := make(map[string]*nodeConn, len(f.nodeConns))
	for tag, conn := range f.nodeConns {
		conns[tag] = conn
	}
	return conns
}

// --------------------------------------------beacon----------------------------------------------

func (f *forwarder) SetMaxBeaconConns(n int) error {
	if n < 64 {
		return errors.New("max beacon connection must >= 64")
	}
	f.maxBeaconConns.Store(n)
	return nil
}

func (f *forwarder) GetMaxBeaconConns() int {
	return f.maxBeaconConns.Load().(int)
}

func (f *forwarder) RegisterBeacon(tag string, conn *beaconConn) error {
	f.beaconConnsRWM.Lock()
	defer f.beaconConnsRWM.Unlock()
	if len(f.beaconConns) >= f.GetMaxBeaconConns() {
		return errors.New("max beacon connections")
	}
	if _, ok := f.beaconConns[tag]; ok {
		return errors.Errorf("beacon has been register\ntag: %s", tag)
	}
	f.beaconConns[tag] = conn
	return nil
}

func (f *forwarder) LogoffBeacon(tag string) {
	f.beaconConnsRWM.Lock()
	defer f.beaconConnsRWM.Unlock()
	if _, ok := f.beaconConns[tag]; ok {
		delete(f.beaconConns, tag)
	}
}

func (f *forwarder) GetBeaconConns() map[string]*beaconConn {
	f.beaconConnsRWM.RLock()
	defer f.beaconConnsRWM.RUnlock()
	conns := make(map[string]*beaconConn, len(f.beaconConns))
	for tag, conn := range f.beaconConns {
		conns[tag] = conn
	}
	return conns
}

func (f *forwarder) log(l logger.Level, log ...interface{}) {
	f.ctx.logger.Println(l, "forwarder", log...)
}

// getConnsExceptBeacon will ger controller, node and client connections
// if connection's tag = except, this connection will not add to the map
func (f *forwarder) getConnsExceptBeacon(except string) map[string]*conn {
	ctrlConns := f.GetCtrlConns()
	nodeConns := f.GetNodeConns()
	clientConns := f.GetClientConns()
	var l int
	if except != "" {
		l = len(ctrlConns) + len(nodeConns) + len(clientConns) - 1
	} else {
		l = len(ctrlConns) + len(nodeConns) + len(clientConns)
	}
	if l < 1 {
		return nil
	}
	allConns := make(map[string]*conn, l)
	for tag, ctrl := range ctrlConns {
		if tag != except {
			allConns[tag] = ctrl.Conn
		}
	}
	for tag, node := range nodeConns {
		if tag != except {
			allConns[tag] = node.Conn
		}
	}
	for tag, client := range clientConns {
		if tag != except {
			allConns[tag] = client.Conn
		}
	}
	return allConns
}

// Send will send controllers, nodes and clients
func (f *forwarder) Send(guid, data []byte, except string, wait bool) ([]*protocol.SendResponse, int) {
	conns := f.getConnsExceptBeacon(except)
	l := len(conns)
	if l == 0 {
		return nil, 0
	}
	var (
		response chan *protocol.SendResponse
		guidCp   []byte
		dataCp   []byte
	)
	if wait {
		response = make(chan *protocol.SendResponse, l)
	} else {
		guidCp = make([]byte, len(guid))
		copy(guidCp, guid)
		dataCp = make([]byte, len(data))
		copy(dataCp, guid)
	}
	for _, c := range conns {
		go func(c *conn) {
			defer func() {
				if r := recover(); r != nil {
					f.log(logger.Fatal, xpanic.Print(r, "forwarder.Send"))
				}
			}()
			if wait {
				response <- c.Send(guid, data)
			} else {
				c.Send(guidCp, dataCp)
			}
		}(c)
	}
	if !wait {
		return nil, 0
	}
	var success int
	responses := make([]*protocol.SendResponse, l)
	for i := 0; i < l; i++ {
		responses[i] = <-response
		if responses[i].Err == nil {
			success++
		}
	}
	close(response)
	return responses, success
}

// Acknowledge will send controllers, nodes and clients
func (f *forwarder) Acknowledge(guid, data []byte, except string, wait bool) (
	[]*protocol.AcknowledgeResponse, int) {
	conns := f.getConnsExceptBeacon(except)
	l := len(conns)
	if l == 0 {
		return nil, 0
	}
	var (
		response chan *protocol.AcknowledgeResponse
		guidCp   []byte
		dataCp   []byte
	)
	if wait {
		response = make(chan *protocol.AcknowledgeResponse, l)
	} else {
		guidCp = make([]byte, len(guid))
		copy(guidCp, guid)
		dataCp = make([]byte, len(data))
		copy(dataCp, guid)
	}
	for _, c := range conns {
		go func(c *conn) {
			defer func() {
				if r := recover(); r != nil {
					f.log(logger.Fatal, xpanic.Print(r, "forwarder.Acknowledge"))
				}
			}()
			if wait {
				response <- c.Acknowledge(guid, data)
			} else {
				c.Acknowledge(guidCp, dataCp)
			}
		}(c)
	}
	if !wait {
		return nil, 0
	}
	var success int
	responses := make([]*protocol.AcknowledgeResponse, l)
	for i := 0; i < l; i++ {
		responses[i] = <-response
		if responses[i].Err == nil {
			success++
		}
	}
	close(response)
	return responses, success
}

func (f *forwarder) Close() {
	close(f.stopSignal)
	f.ctx = nil
}
