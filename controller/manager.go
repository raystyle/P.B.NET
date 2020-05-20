package controller

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/dns"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/option"
	"project/internal/xpanic"
)

// clientMgr contains all clients from NewClient() and client options from Config.
// it can generate client tag, you can manage all clients here.
type clientMgr struct {
	ctx *Ctrl

	// options from Config
	timeout   time.Duration
	proxyTag  string
	dnsOpts   dns.Options
	tlsConfig option.TLSConfig
	optsRWM   sync.RWMutex

	guid *guid.Generator

	clients    map[guid.GUID]*Client
	clientsRWM sync.RWMutex
}

func newClientManager(ctx *Ctrl, config *Config) (*clientMgr, error) {
	cfg := config.Client

	if cfg.Timeout < 10*time.Second {
		return nil, errors.New("client timeout must >= 10 seconds")
	}

	mgr := &clientMgr{
		ctx:       ctx,
		timeout:   cfg.Timeout,
		proxyTag:  cfg.ProxyTag,
		dnsOpts:   cfg.DNSOpts,
		tlsConfig: cfg.TLSConfig,
		guid:      guid.New(4, ctx.global.Now),
		clients:   make(map[guid.GUID]*Client),
	}
	mgr.tlsConfig.CertPool = ctx.global.CertPool
	return mgr, nil
}

func (mgr *clientMgr) GetTimeout() time.Duration {
	mgr.optsRWM.RLock()
	defer mgr.optsRWM.RUnlock()
	return mgr.timeout
}

func (mgr *clientMgr) GetProxyTag() string {
	mgr.optsRWM.RLock()
	defer mgr.optsRWM.RUnlock()
	return mgr.proxyTag
}

func (mgr *clientMgr) GetDNSOptions() *dns.Options {
	mgr.optsRWM.RLock()
	defer mgr.optsRWM.RUnlock()
	return mgr.dnsOpts.Clone()
}

func (mgr *clientMgr) GetTLSConfig() *option.TLSConfig {
	mgr.optsRWM.RLock()
	defer mgr.optsRWM.RUnlock()
	return &mgr.tlsConfig
}

func (mgr *clientMgr) SetTimeout(timeout time.Duration) error {
	if timeout < 10*time.Second {
		return errors.New("timeout must >= 10 seconds")
	}
	mgr.optsRWM.Lock()
	defer mgr.optsRWM.Unlock()
	mgr.timeout = timeout
	return nil
}

func (mgr *clientMgr) SetProxyTag(tag string) error {
	// check proxy is exist
	_, err := mgr.ctx.global.ProxyPool.Get(tag)
	if err != nil {
		return err
	}
	mgr.optsRWM.Lock()
	defer mgr.optsRWM.Unlock()
	mgr.proxyTag = tag
	return nil
}

func (mgr *clientMgr) SetDNSOptions(opts *dns.Options) {
	mgr.optsRWM.Lock()
	defer mgr.optsRWM.Unlock()
	mgr.dnsOpts = *opts.Clone()
}

func (mgr *clientMgr) SetTLSConfig(cfg *option.TLSConfig) error {
	_, err := cfg.Apply()
	if err != nil {
		return errors.WithStack(err)
	}
	mgr.optsRWM.Lock()
	defer mgr.optsRWM.Unlock()
	mgr.tlsConfig = *cfg
	mgr.tlsConfig.CertPool = mgr.ctx.global.CertPool
	return nil
}

// for NewClient()
func (mgr *clientMgr) Add(client *Client) {
	client.tag = mgr.guid.Get()
	mgr.clientsRWM.Lock()
	defer mgr.clientsRWM.Unlock()
	if _, ok := mgr.clients[*client.tag]; !ok {
		mgr.clients[*client.tag] = client
	}
}

// for client.Close().
func (mgr *clientMgr) Delete(tag *guid.GUID) {
	mgr.clientsRWM.Lock()
	defer mgr.clientsRWM.Unlock()
	delete(mgr.clients, *tag)
}

// Clients is used to get all clients
func (mgr *clientMgr) Clients() map[guid.GUID]*Client {
	mgr.clientsRWM.RLock()
	defer mgr.clientsRWM.RUnlock()
	clients := make(map[guid.GUID]*Client, len(mgr.clients))
	for tag, client := range mgr.clients {
		clients[tag] = client
	}
	return clients
}

// Kill is used to close client. Must use cm.Clients(),
// because client.Close() will use cm.clientsRWM.
func (mgr *clientMgr) Kill(tag *guid.GUID) {
	if client, ok := mgr.Clients()[*tag]; ok {
		client.Close()
	}
}

// Close will close all active clients.
func (mgr *clientMgr) Close() {
	for {
		for _, client := range mgr.Clients() {
			client.Close()
		}
		time.Sleep(10 * time.Millisecond)
		if len(mgr.Clients()) == 0 {
			break
		}
	}
	mgr.guid.Close()
	mgr.ctx = nil
}

// about messageMgr
type roleMessageSlot struct {
	slots map[guid.GUID]chan interface{}
	rwm   sync.RWMutex
}

// messageMgr is used to manage messages that send to Node and Beacon.
// It will return the reply about the message.
type messageMgr struct {
	ctx *Ctrl

	// 2 * sender.Timeout
	timeout time.Duration

	nodeSlots      map[guid.GUID]*roleMessageSlot
	nodeSlotsRWM   sync.RWMutex
	beaconSlots    map[guid.GUID]*roleMessageSlot
	beaconSlotsRWM sync.RWMutex

	slotPool  sync.Pool
	timerPool sync.Pool

	guid *guid.Generator

	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newMessageManager(ctx *Ctrl, config *Config) *messageMgr {
	cfg := config.Sender

	mgr := messageMgr{
		ctx:         ctx,
		timeout:     2 * cfg.Timeout,
		nodeSlots:   make(map[guid.GUID]*roleMessageSlot),
		beaconSlots: make(map[guid.GUID]*roleMessageSlot),
		guid:        guid.New(1024, ctx.global.Now),
	}
	mgr.slotPool.New = func() interface{} {
		return make(chan interface{}, 1)
	}
	mgr.timerPool.New = func() interface{} {
		timer := time.NewTimer(time.Minute)
		timer.Stop()
		return timer
	}
	mgr.context, mgr.cancel = context.WithCancel(context.Background())
	mgr.wg.Add(1)
	go mgr.cleaner()
	return &mgr
}

func (mgr *messageMgr) mustGetNodeSlot(role *guid.GUID) *roleMessageSlot {
	mgr.nodeSlotsRWM.Lock()
	defer mgr.nodeSlotsRWM.Unlock()
	ns := mgr.nodeSlots[*role]
	if ns != nil {
		return ns
	}
	rms := &roleMessageSlot{
		slots: make(map[guid.GUID]chan interface{}),
		rwm:   sync.RWMutex{},
	}
	mgr.nodeSlots[*role] = rms
	return rms
}

func (mgr *messageMgr) createNodeSlot(role *guid.GUID) (*guid.GUID, chan interface{}) {
	id := mgr.guid.Get()
	ch := mgr.slotPool.Get().(chan interface{})
	ns := mgr.mustGetNodeSlot(role)
	ns.rwm.Lock()
	defer ns.rwm.Unlock()
	ns.slots[*id] = ch
	return id, ch
}

func (mgr *messageMgr) destroyNodeSlot(role, id *guid.GUID, ch chan interface{}) {
	ns := mgr.mustGetNodeSlot(role)
	ns.rwm.Lock()
	defer ns.rwm.Unlock()
	// when read channel timeout, defer call destroySlot(),
	// the channel maybe has reply, try to clean it.
	select {
	case <-ch:
	default:
	}
	mgr.slotPool.Put(ch)
	delete(ns.slots, *id)
}

func (mgr *messageMgr) mustGetBeaconSlot(role *guid.GUID) *roleMessageSlot {
	mgr.beaconSlotsRWM.Lock()
	defer mgr.beaconSlotsRWM.Unlock()
	ns := mgr.beaconSlots[*role]
	if ns != nil {
		return ns
	}
	rms := &roleMessageSlot{
		slots: make(map[guid.GUID]chan interface{}),
		rwm:   sync.RWMutex{},
	}
	mgr.beaconSlots[*role] = rms
	return rms
}

func (mgr *messageMgr) createBeaconSlot(role *guid.GUID) (*guid.GUID, chan interface{}) {
	id := mgr.guid.Get()
	ch := mgr.slotPool.Get().(chan interface{})
	bs := mgr.mustGetBeaconSlot(role)
	bs.rwm.Lock()
	defer bs.rwm.Unlock()
	bs.slots[*id] = ch
	return id, ch
}

func (mgr *messageMgr) destroyBeaconSlot(role, id *guid.GUID, ch chan interface{}) {
	bs := mgr.mustGetBeaconSlot(role)
	bs.rwm.Lock()
	defer bs.rwm.Unlock()
	// when read channel timeout, defer call destroySlot(),
	// the channel maybe has reply, try to clean it.
	select {
	case <-ch:
	default:
	}
	mgr.slotPool.Put(ch)
	delete(bs.slots, *id)
}

// SendToNode is used to send message to Node and get the reply.
func (mgr *messageMgr) SendToNode(
	ctx context.Context,
	guid *guid.GUID,
	command []byte,
	message messages.RoundTripper,
	deflate bool,
	timeout time.Duration,
) (interface{}, error) {
	// set message id
	id, reply := mgr.createNodeSlot(guid)
	defer mgr.destroyNodeSlot(guid, id, reply)
	message.SetID(id)
	// send
	err := mgr.ctx.sender.SendToNode(ctx, guid, command, message, deflate)
	if err != nil {
		return nil, err
	}
	// get reply
	timer := mgr.timerPool.Get().(*time.Timer)
	defer mgr.timerPool.Put(timer)
	if timeout < 1 {
		timeout = mgr.timeout
	}
	timer.Reset(timeout)
	select {
	case resp := <-reply:
		if !timer.Stop() {
			<-timer.C
		}
		return resp, nil
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return nil, ctx.Err()
	case <-timer.C:
		return nil, errors.New("get reply timeout about send to node")
	}
}

// SendToBeacon is used to send message to Beacon and get the reply.
// If Beacon not in interactive mode, these message will insert to
// database and wait Beacon to query it.
func (mgr *messageMgr) SendToBeacon(
	ctx context.Context,
	guid *guid.GUID,
	command []byte,
	message messages.RoundTripper,
	deflate bool,
	timeout time.Duration,
) (interface{}, error) {
	if !mgr.ctx.sender.IsInInteractiveMode(guid) {
		return nil, mgr.ctx.sender.SendToBeacon(ctx, guid, command, message, deflate)
	}
	// set message id
	id, reply := mgr.createBeaconSlot(guid)
	defer mgr.destroyBeaconSlot(guid, id, reply)
	message.SetID(id)
	// send
	err := mgr.ctx.sender.SendToBeacon(ctx, guid, command, message, deflate)
	if err != nil {
		return nil, err
	}
	// get reply
	timer := mgr.timerPool.Get().(*time.Timer)
	defer mgr.timerPool.Put(timer)
	if timeout < 1 {
		timeout = mgr.timeout
	}
	timer.Reset(timeout)
	select {
	case resp := <-reply:
		if !timer.Stop() {
			<-timer.C
		}
		return resp, nil
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return nil, ctx.Err()
	case <-timer.C:
		return nil, errors.New("get reply timeout about send to beacon")
	}
}

// SendToNodeFromPlugin is used to send message to Node and get the reply.
func (mgr *messageMgr) SendToNodeFromPlugin(
	guid *guid.GUID,
	command []byte,
	message []byte,
	deflate bool,
	timeout time.Duration,
) ([]byte, error) {
	request := &messages.PluginRequest{
		Request: message,
	}
	reply, err := mgr.SendToNode(mgr.context, guid, command, request, deflate, timeout)
	if err != nil {
		return nil, err
	}
	return reply.([]byte), nil
}

// SendToBeaconFromPlugin is used to send message to Beacon and get the reply.
func (mgr *messageMgr) SendToBeaconFromPlugin(
	guid *guid.GUID,
	command []byte,
	message []byte,
	deflate bool,
	timeout time.Duration,
) ([]byte, error) {
	request := &messages.PluginRequest{
		Request: message,
	}
	reply, err := mgr.SendToBeacon(mgr.context, guid, command, request, deflate, timeout)
	if err != nil {
		return nil, err
	}
	return reply.([]byte), nil
}

func (mgr *messageMgr) getNodeSlot(role *guid.GUID) *roleMessageSlot {
	mgr.nodeSlotsRWM.RLock()
	defer mgr.nodeSlotsRWM.RUnlock()
	return mgr.nodeSlots[*role]
}

// HandleNodeReply is used to set Node reply, handler.Handle functions will call it.
func (mgr *messageMgr) HandleNodeReply(role, id *guid.GUID, reply interface{}) {
	ns := mgr.getNodeSlot(role)
	if ns == nil {
		return
	}
	ns.rwm.RLock()
	defer ns.rwm.RUnlock()
	if ch, ok := ns.slots[*id]; ok {
		select {
		case ch <- reply:
		default:
		}
	}
}

func (mgr *messageMgr) getBeaconSlot(role *guid.GUID) *roleMessageSlot {
	mgr.beaconSlotsRWM.RLock()
	defer mgr.beaconSlotsRWM.RUnlock()
	return mgr.beaconSlots[*role]
}

// HandleBeaconReply is used to set Beacon reply, handler.Handle functions will call it.
func (mgr *messageMgr) HandleBeaconReply(role, id *guid.GUID, reply interface{}) {
	if id.IsZero() {
		return
	}
	bs := mgr.getBeaconSlot(role)
	if bs == nil {
		return
	}
	bs.rwm.RLock()
	defer bs.rwm.RUnlock()
	if ch, ok := bs.slots[*id]; ok {
		select {
		case ch <- reply:
		default:
		}
	}
}

func (mgr *messageMgr) cleaner() {
	defer func() {
		if r := recover(); r != nil {
			buf := xpanic.Print(r, "messageMgr.cleaner")
			mgr.ctx.logger.Print(logger.Fatal, "message-manager", buf)
			// restart message cleaner
			time.Sleep(time.Second)
			go mgr.cleaner()
		} else {
			mgr.wg.Done()
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			mgr.cleanNodeSlotMap()
			mgr.cleanBeaconSlotMap()
		case <-mgr.context.Done():
			return
		}
	}
}

func (mgr *messageMgr) cleanNodeSlotMap() {
	mgr.nodeSlotsRWM.Lock()
	defer mgr.nodeSlotsRWM.Unlock()
	newMap := make(map[guid.GUID]*roleMessageSlot)
	for key, ns := range mgr.nodeSlots {
		if mgr.cleanRoleSlotMap(ns) {
			newMap[key] = ns
		}
	}
	mgr.nodeSlots = newMap
}

func (mgr *messageMgr) cleanBeaconSlotMap() {
	mgr.beaconSlotsRWM.Lock()
	defer mgr.beaconSlotsRWM.Unlock()
	newMap := make(map[guid.GUID]*roleMessageSlot)
	for key, bs := range mgr.beaconSlots {
		if mgr.cleanRoleSlotMap(bs) {
			newMap[key] = bs
		}
	}
	mgr.beaconSlots = newMap
}

// delete zero length map or allocate a new slots map
func (mgr *messageMgr) cleanRoleSlotMap(rms *roleMessageSlot) bool {
	rms.rwm.Lock()
	defer rms.rwm.Unlock()
	l := len(rms.slots)
	if l == 0 {
		return false
	}
	newMap := make(map[guid.GUID]chan interface{}, l)
	for id, message := range rms.slots {
		newMap[id] = message
	}
	rms.slots = newMap
	return true
}

func (mgr *messageMgr) Close() {
	mgr.cancel()
	mgr.wg.Wait()
	mgr.guid.Close()
	mgr.ctx = nil
}

type action struct {
	object    interface{}
	timeout   int64
	timestamp int64
}

// actionMgr is used to manage event from Node and Beacon,
// and need Controller to interactive it, action manager
// will save the context data with generated GUID.
type actionMgr struct {
	ctx *Ctrl

	// default timeout
	timeout time.Duration

	// key = guid.Hex()
	actions   map[string]*action
	actionsMu sync.Mutex

	guid *guid.Generator

	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newActionManager(ctx *Ctrl, config *Config) *actionMgr {
	cfg := config.Sender

	mgr := actionMgr{
		ctx:     ctx,
		timeout: 2 * cfg.Timeout,
		actions: make(map[string]*action),
		guid:    guid.New(1024, ctx.global.Now),
	}
	mgr.context, mgr.cancel = context.WithCancel(context.Background())
	mgr.wg.Add(1)
	go mgr.cleaner()
	return &mgr
}

// Store is used to store action, it will return action id.
func (mgr *actionMgr) Store(object interface{}, timeout time.Duration) string {
	if timeout < 1 {
		timeout = mgr.timeout
	}
	id := mgr.guid.Get().Hex()
	timestamp := mgr.ctx.global.Now().Unix()
	mgr.actionsMu.Lock()
	defer mgr.actionsMu.Unlock()
	mgr.actions[id] = &action{
		object:    object,
		timeout:   int64(timeout.Seconds()),
		timestamp: timestamp,
	}
	return id
}

// Load is used to load action, it will delete action if it exists.
func (mgr *actionMgr) Load(id string) (interface{}, error) {
	mgr.actionsMu.Lock()
	defer mgr.actionsMu.Unlock()
	if action, ok := mgr.actions[id]; ok {
		delete(mgr.actions, id)
		return action.object, nil
	}
	return nil, errors.New("this action doesn't exist")
}

func (mgr *actionMgr) Close() {
	mgr.cancel()
	mgr.wg.Wait()
	mgr.guid.Close()
	mgr.ctx = nil
}

func (mgr *actionMgr) cleaner() {
	defer func() {
		if r := recover(); r != nil {
			buf := xpanic.Print(r, "actionMgr.cleaner")
			mgr.ctx.logger.Print(logger.Fatal, "action-manager", buf)
			// restart message cleaner
			time.Sleep(time.Second)
			go mgr.cleaner()
		} else {
			mgr.wg.Done()
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			mgr.clean()
		case <-mgr.context.Done():
			return
		}
	}
}

func (mgr *actionMgr) clean() {
	now := mgr.ctx.global.Now().Unix()
	mgr.actionsMu.Lock()
	defer mgr.actionsMu.Unlock()
	newMap := make(map[string]*action, len(mgr.actions))
	for id, action := range mgr.actions {
		if convert.AbsInt64(now-action.timestamp) < action.timeout {
			newMap[id] = action
		}
	}
	mgr.actions = newMap
}
