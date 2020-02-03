package node

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"

	"project/internal/guid"
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

	clientConns    map[guid.GUID]*Client
	clientConnsRWM sync.RWMutex
	ctrlConns      map[guid.GUID]*ctrlConn
	ctrlConnsRWM   sync.RWMutex
	nodeConns      map[guid.GUID]*nodeConn
	nodeConnsRWM   sync.RWMutex
	beaconConns    map[guid.GUID]*beaconConn
	beaconConnsRWM sync.RWMutex

	bufferPool sync.Pool

	stopSignal chan struct{}
	wg         sync.WaitGroup
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
	f.clientConns = make(map[guid.GUID]*Client, cfg.MaxClientConns)
	f.ctrlConns = make(map[guid.GUID]*ctrlConn, cfg.MaxCtrlConns)
	f.nodeConns = make(map[guid.GUID]*nodeConn, cfg.MaxNodeConns)
	f.beaconConns = make(map[guid.GUID]*beaconConn, cfg.MaxBeaconConns)
	f.bufferPool.New = func() interface{} {
		return new(bytes.Buffer)
	}
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

func (f *forwarder) RegisterClient(tag *guid.GUID, client *Client) error {
	f.clientConnsRWM.Lock()
	defer f.clientConnsRWM.Unlock()
	if len(f.clientConns) >= f.GetMaxClientConns() {
		return errors.New("max client connections")
	}
	if _, ok := f.clientConns[*tag]; ok {
		return errors.Errorf("client has been register\n%s", tag)
	}
	f.clientConns[*tag] = client
	return nil
}

func (f *forwarder) LogoffClient(tag *guid.GUID) {
	f.clientConnsRWM.Lock()
	defer f.clientConnsRWM.Unlock()
	if _, ok := f.clientConns[*tag]; ok {
		delete(f.clientConns, *tag)
	}
}

func (f *forwarder) GetClientConns() map[guid.GUID]*Client {
	f.clientConnsRWM.RLock()
	defer f.clientConnsRWM.RUnlock()
	clients := make(map[guid.GUID]*Client, len(f.clientConns))
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

func (f *forwarder) RegisterCtrl(tag *guid.GUID, conn *ctrlConn) error {
	f.ctrlConnsRWM.Lock()
	defer f.ctrlConnsRWM.Unlock()
	if len(f.ctrlConns) >= f.GetMaxCtrlConns() {
		return errors.New("max controller connections")
	}
	if _, ok := f.ctrlConns[*tag]; ok {
		return errors.Errorf("controller has been register\n%s", tag)
	}
	f.ctrlConns[*tag] = conn
	return nil
}

func (f *forwarder) LogoffCtrl(tag *guid.GUID) {
	f.ctrlConnsRWM.Lock()
	defer f.ctrlConnsRWM.Unlock()
	if _, ok := f.ctrlConns[*tag]; ok {
		delete(f.ctrlConns, *tag)
	}
}

func (f *forwarder) GetCtrlConns() map[guid.GUID]*ctrlConn {
	f.ctrlConnsRWM.RLock()
	defer f.ctrlConnsRWM.RUnlock()
	conns := make(map[guid.GUID]*ctrlConn, len(f.ctrlConns))
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

func (f *forwarder) RegisterNode(tag *guid.GUID, conn *nodeConn) error {
	f.nodeConnsRWM.Lock()
	defer f.nodeConnsRWM.Unlock()
	if len(f.nodeConns) >= f.GetMaxNodeConns() {
		return errors.New("max node connections")
	}
	if _, ok := f.nodeConns[*tag]; ok {
		return errors.Errorf("node has been register\n%s", tag)
	}
	f.nodeConns[*tag] = conn
	return nil
}

func (f *forwarder) LogoffNode(tag *guid.GUID) {
	f.nodeConnsRWM.Lock()
	defer f.nodeConnsRWM.Unlock()
	if _, ok := f.nodeConns[*tag]; ok {
		delete(f.nodeConns, *tag)
	}
}

func (f *forwarder) GetNodeConns() map[guid.GUID]*nodeConn {
	f.nodeConnsRWM.RLock()
	defer f.nodeConnsRWM.RUnlock()
	conns := make(map[guid.GUID]*nodeConn, len(f.nodeConns))
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

func (f *forwarder) RegisterBeacon(tag *guid.GUID, conn *beaconConn) error {
	f.beaconConnsRWM.Lock()
	defer f.beaconConnsRWM.Unlock()
	if len(f.beaconConns) >= f.GetMaxBeaconConns() {
		return errors.New("max beacon connections")
	}
	if _, ok := f.beaconConns[*tag]; ok {
		return errors.Errorf("beacon has been register\n%s", tag)
	}
	f.beaconConns[*tag] = conn
	return nil
}

func (f *forwarder) LogoffBeacon(tag *guid.GUID) {
	f.beaconConnsRWM.Lock()
	defer f.beaconConnsRWM.Unlock()
	if _, ok := f.beaconConns[*tag]; ok {
		delete(f.beaconConns, *tag)
	}
}

func (f *forwarder) GetBeaconConns() map[guid.GUID]*beaconConn {
	f.beaconConnsRWM.RLock()
	defer f.beaconConnsRWM.RUnlock()
	conns := make(map[guid.GUID]*beaconConn, len(f.beaconConns))
	for tag, conn := range f.beaconConns {
		conns[tag] = conn
	}
	return conns
}

func (f *forwarder) log(l logger.Level, log ...interface{}) {
	f.ctx.logger.Println(l, "forwarder", log...)
}

func (f *forwarder) forward(
	conns map[guid.GUID]*conn,
	number int,
	operation uint8,
	guid *guid.GUID,
	data []byte,
) {
	// get cache
	guidBuf := f.bufferPool.Get().(*bytes.Buffer)
	guidBuf.Reset()
	guidBuf.Write(guid[:])
	guidBytes := guidBuf.Bytes()

	dataBuf := f.bufferPool.Get().(*bytes.Buffer)
	dataBuf.Reset()
	dataBuf.Write(data)
	dataBytes := dataBuf.Bytes()

	done := make(chan struct{}, number)
	f.wg.Add(number + 1)
	for _, c := range conns {
		go func(c *conn) {
			defer func() {
				if r := recover(); r != nil {
					f.log(logger.Fatal, xpanic.Print(r, "forwarder.forward"))
				}
				f.wg.Done()
			}()
			f.operate(c, operation, guidBytes, dataBytes)
			select {
			case done <- struct{}{}:
			case <-f.stopSignal:
			}
		}(c)
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				f.log(logger.Fatal, xpanic.Print(r, "forwarder.forward"))
			}
			f.wg.Done()
		}()
		// wait forward
		for i := 0; i < number; i++ {
			select {
			case <-done:
			case <-f.stopSignal:
				return
			}
		}
		close(done)
		f.bufferPool.Put(guidBuf)
		f.bufferPool.Put(dataBuf)
	}()
}

func (f *forwarder) operate(conn *conn, operation uint8, guid, data []byte) {
	switch operation {
	case protocol.CtrlBroadcast:
		conn.Broadcast(guid, data)
	case protocol.CtrlSendToNode:
		conn.SendToNode(guid, data)
	case protocol.CtrlAckToNode:
		conn.AckToNode(guid, data)
	case protocol.NodeSend:
		conn.NodeSend(guid, data)
	case protocol.NodeAck:
		conn.NodeAck(guid, data)
	default:
		panic(fmt.Sprintf("unknown operation: %d", operation))
	}
}

// getConnsExceptCtrlBeaconAndIncome will get Node and Client connections
// if income connection's tag = except, this connection will not add to the map
func (f *forwarder) getConnsExceptCtrlBeaconAndIncome(except *guid.GUID) map[guid.GUID]*conn {
	nodeConns := f.GetNodeConns()
	clientConns := f.GetClientConns()
	l := len(nodeConns) + len(clientConns) // not -1, because except maybe Controller
	if l == 0 {
		return nil
	}
	allConns := make(map[guid.GUID]*conn, l)
	for tag, node := range nodeConns {
		if tag != *except {
			allConns[tag] = node.Conn
		}
	}
	for tag, client := range clientConns {
		if tag != *except {
			allConns[tag] = client.Conn
		}
	}
	return allConns
}

// Broadcast is used to forward Controller Broadcast message to Node and Client
// it will not block
func (f *forwarder) Broadcast(guid *guid.GUID, data []byte, except *guid.GUID) {
	conns := f.getConnsExceptCtrlBeaconAndIncome(except)
	l := len(conns)
	if l == 0 {
		return
	}
	f.forward(conns, l, protocol.CtrlBroadcast, guid, data)
}

// SendToNode is used to forward Controller SendToNode message to Node and Client
// it will not block
func (f *forwarder) SendToNode(guid *guid.GUID, data []byte, except *guid.GUID) {
	conns := f.getConnsExceptCtrlBeaconAndIncome(except)
	l := len(conns)
	if l == 0 {
		return
	}
	f.forward(conns, l, protocol.CtrlSendToNode, guid, data)
}

// AckToNode is used to forward Controller AckToNode message to Node and Client
// it will not block
func (f *forwarder) AckToNode(guid *guid.GUID, data []byte, except *guid.GUID) {
	conns := f.getConnsExceptCtrlBeaconAndIncome(except)
	l := len(conns)
	if l == 0 {
		return
	}
	f.forward(conns, l, protocol.CtrlAckToNode, guid, data)
}

// getConnsExceptBeaconAndIncome will get Controller, Node and Client connections
// if income connection's tag = except, this connection will not add to the map
func (f *forwarder) getConnsExceptBeaconAndIncome(except *guid.GUID) map[guid.GUID]*conn {
	ctrlConns := f.GetCtrlConns()
	nodeConns := f.GetNodeConns()
	clientConns := f.GetClientConns()
	l := len(ctrlConns) + len(nodeConns) + len(clientConns) - 1
	if l == 0 {
		return nil
	}
	allConns := make(map[guid.GUID]*conn, l)
	for tag, ctrl := range ctrlConns {
		allConns[tag] = ctrl.Conn
	}
	for tag, node := range nodeConns {
		if tag != *except {
			allConns[tag] = node.Conn
		}
	}
	for tag, client := range clientConns {
		if tag != *except {
			allConns[tag] = client.Conn
		}
	}
	return allConns
}

// NodeSend is used to forward Node send to Controller
// it will not block
func (f *forwarder) NodeSend(guid *guid.GUID, data []byte, except *guid.GUID) {
	conns := f.getConnsExceptBeaconAndIncome(except)
	l := len(conns)
	if l == 0 {
		return
	}
	f.forward(conns, l, protocol.NodeSend, guid, data)
}

// NodeAck is used to forward Node acknowledge to Controller
// it will not block
func (f *forwarder) NodeAck(guid *guid.GUID, data []byte, except *guid.GUID) {
	conns := f.getConnsExceptBeaconAndIncome(except)
	l := len(conns)
	if l == 0 {
		return
	}
	f.forward(conns, l, protocol.NodeAck, guid, data)
}

// getConnsExceptBeacon will get Controller, Node and Client connections
func (f *forwarder) getConnsExceptBeacon() map[guid.GUID]*conn {
	ctrlConns := f.GetCtrlConns()
	nodeConns := f.GetNodeConns()
	clientConns := f.GetClientConns()
	l := len(ctrlConns) + len(nodeConns) + len(clientConns)
	if l == 0 {
		return nil
	}
	allConns := make(map[guid.GUID]*conn, l)
	for tag, ctrl := range ctrlConns {
		allConns[tag] = ctrl.Conn
	}
	for tag, node := range nodeConns {
		allConns[tag] = node.Conn
	}
	for tag, client := range clientConns {
		allConns[tag] = client.Conn
	}
	return allConns
}

// Send will send Controllers, Nodes and Clients, sender need it.
// it will block until get all response.
func (f *forwarder) Send(
	guid *guid.GUID,
	data *bytes.Buffer,
) ([]*protocol.SendResponse, int) {
	conns := f.getConnsExceptBeacon()
	l := len(conns)
	if l == 0 {
		return nil, 0
	}
	response := make(chan *protocol.SendResponse, l)
	for _, c := range conns {
		go func(c *conn) {
			defer func() {
				if r := recover(); r != nil {
					f.log(logger.Fatal, xpanic.Print(r, "forwarder.Send"))
				}
			}()
			response <- c.Send(guid, data)
		}(c)
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

// Acknowledge will acknowledge Controllers, Nodes and Clients, sender need it.
// it will block until get all response.
func (f *forwarder) Acknowledge(
	guid *guid.GUID,
	data *bytes.Buffer,
) ([]*protocol.AcknowledgeResponse, int) {
	conns := f.getConnsExceptBeacon()
	l := len(conns)
	if l == 0 {
		return nil, 0
	}
	response := make(chan *protocol.AcknowledgeResponse, l)
	for _, c := range conns {
		go func(c *conn) {
			defer func() {
				if r := recover(); r != nil {
					f.log(logger.Fatal, xpanic.Print(r, "forwarder.Acknowledge"))
				}
			}()
			response <- c.Acknowledge(guid, data)
		}(c)
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
	f.wg.Wait()
	f.ctx = nil
}
