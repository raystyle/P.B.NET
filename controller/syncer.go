package controller

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

const (
	syncerNode   = 0
	syncerBeacon = 1
)

type syncer struct {
	ctx              *CTRL
	maxBufferSize    int
	maxSyncer        int
	retryTimes       int
	retryInterval    time.Duration
	broadcastTimeout float64
	configsM         sync.Mutex
	// -------------------handle broadcast------------------------
	// key=base64(guid) value=timestamp, check whether handled
	broadcastQueue   chan *protocol.Broadcast
	broadcastGUID    [2]map[string]int64
	broadcastGUIDRWM [2]sync.RWMutex
	// -----------------handle sync message-----------------------
	syncSendQueue      chan *protocol.SyncSend
	syncSendGUID       [2]map[string]int64
	syncSendGUIDRWM    [2]sync.RWMutex
	syncReceiveQueue   chan *protocol.SyncReceive
	syncReceiveGUID    [2]map[string]int64
	syncReceiveGUIDRWM [2]sync.RWMutex
	// -------------------handle sync task------------------------
	syncTaskQueue chan *protocol.SyncTask
	// check is sync
	syncStatus   [2]map[string]struct{}
	syncStatusM  [2]sync.Mutex
	blockWorker  int
	blockWorkerM sync.Mutex
	// runtime
	sClients    map[string]*sClient
	sClientsRWM sync.RWMutex
	stopSignal  chan struct{}
	wg          sync.WaitGroup
}

func newSyncer(ctx *CTRL, cfg *Config) (*syncer, error) {
	// check config
	if cfg.MaxBufferSize < 4096 {
		return nil, errors.New("max buffer size < 4096")
	}
	if cfg.MaxSyncer < 1 {
		return nil, errors.New("max syncer < 1")
	}
	if cfg.WorkerNumber < 2 {
		return nil, errors.New("worker number < 2")
	}
	if cfg.WorkerQueueSize < 512 {
		return nil, errors.New("worker task queue size < 512")
	}
	if cfg.ReserveWorker >= cfg.WorkerNumber {
		return nil, errors.New("reserve worker number >= worker number")
	}
	if cfg.RetryTimes < 3 {
		return nil, errors.New("retry time < 3")
	}
	if cfg.RetryInterval < 5*time.Second {
		return nil, errors.New("retry interval < 5s")
	}
	if cfg.BroadcastTimeout < 30*time.Second {
		return nil, errors.New("broadcast timeout < 30s")
	}
	syncer := syncer{
		ctx:              ctx,
		maxBufferSize:    cfg.MaxBufferSize,
		maxSyncer:        cfg.MaxSyncer,
		retryTimes:       cfg.RetryTimes,
		retryInterval:    cfg.RetryInterval,
		broadcastTimeout: cfg.BroadcastTimeout.Seconds(),
		sClients:         make(map[string]*sClient),
	}

	return &syncer, nil
}

func (syncer *syncer) Close() {

}

func (syncer *syncer) Connect(cfg *clientCfg) {

}

// task from syncer client

func (syncer *syncer) addBroadcast(br *protocol.Broadcast) {

}

func (syncer *syncer) addSyncSend(ss *protocol.SyncSend) {

}

func (syncer *syncer) addSyncReceive(sr *protocol.SyncReceive) {

}

// check xxx Token is used to check xxx is been handled
// xxx = broadcast, sync send, sync receive
// just tell others, but they can still send it by force

func (syncer *syncer) checkBroadcastToken(role protocol.Role, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.broadcastGUIDRWM[i].RLock()
	_, ok := syncer.broadcastGUID[i][key]
	syncer.broadcastGUIDRWM[i].RUnlock()
	return !ok
}

func (syncer *syncer) checkSyncSendToken(role protocol.Role, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.syncSendGUIDRWM[i].RLock()
	_, ok := syncer.syncSendGUID[i][key]
	syncer.syncSendGUIDRWM[i].RUnlock()
	return !ok
}

func (syncer *syncer) checkSyncReceiveToken(role protocol.Role, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.syncReceiveGUIDRWM[i].RLock()
	_, ok := syncer.syncReceiveGUID[i][key]
	syncer.syncReceiveGUIDRWM[i].RUnlock()
	return !ok
}

// check xxx GUID is used to check xxx is been handled
// prevent others send same message
// xxx = broadcast, sync send, sync receive

func (syncer *syncer) checkBroadcastGUID(role protocol.Role, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.broadcastGUIDRWM[i].Lock()
	if _, ok := syncer.broadcastGUID[i][key]; !ok {
		syncer.broadcastGUID[i][key] = timestamp
		syncer.broadcastGUIDRWM[i].Unlock()
		return true
	} else {
		syncer.broadcastGUIDRWM[i].Unlock()
		return false
	}
}

func (syncer *syncer) checkSyncSendGUID(role protocol.Role, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.syncSendGUIDRWM[i].Lock()
	if _, ok := syncer.syncSendGUID[i][key]; !ok {
		syncer.syncSendGUID[i][key] = timestamp
		syncer.syncSendGUIDRWM[i].Unlock()
		return true
	} else {
		syncer.syncSendGUIDRWM[i].Unlock()
		return false
	}
}

func (syncer *syncer) checkSyncReceiveGUID(role protocol.Role, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.syncReceiveGUIDRWM[i].Lock()
	if _, ok := syncer.syncReceiveGUID[i][key]; !ok {
		syncer.syncReceiveGUID[i][key] = timestamp
		syncer.syncReceiveGUIDRWM[i].Unlock()
		return true
	} else {
		syncer.syncReceiveGUIDRWM[i].Unlock()
		return false
	}
}

// guidCleaner is use to clean expire guid
func (syncer *syncer) guidCleaner() {
	defer syncer.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := syncer.ctx.global.Now().Unix()
			for i := 0; i < 2; i++ {
				// clean broadcast
				syncer.broadcastGUIDRWM[i].Lock()
				for key, timestamp := range syncer.broadcastGUID[i] {
					if float64(now-timestamp) > syncer.broadcastTimeout {
						delete(syncer.broadcastGUID[i], key)
					}
				}
				syncer.broadcastGUIDRWM[i].Unlock()
				// clean sync send
				syncer.syncSendGUIDRWM[i].Lock()
				for key, timestamp := range syncer.syncSendGUID[i] {
					if float64(now-timestamp) > syncer.broadcastTimeout {
						delete(syncer.syncSendGUID[i], key)
					}
				}
				syncer.syncSendGUIDRWM[i].Unlock()
				// clean sync receive
				syncer.syncReceiveGUIDRWM[i].Lock()
				for key, timestamp := range syncer.syncReceiveGUID[i] {
					if float64(now-timestamp) > syncer.broadcastTimeout {
						delete(syncer.syncReceiveGUID[i], key)
					}
				}
				syncer.syncReceiveGUIDRWM[i].Unlock()
			}
		case <-syncer.stopSignal:
			return
		}
	}
}

// syncer client
type sClient struct {
	ctx    *syncer
	guid   []byte
	client *client
}

func newSClient(ctx *syncer, cfg *clientCfg) (*sClient, error) {
	sc := sClient{
		ctx: ctx,
	}
	cfg.MsgHandler = sc.handleMessage
	client, err := newClient(ctx.ctx, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "new syncer client failed")
	}
	sc.guid = cfg.NodeGUID
	sc.client = client
	// start handle
	// <warning> not add wg
	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := xpanic.Error("syncer client panic:", r)
				client.log(logger.FATAL, err)
			}
			client.Close()
		}()
		protocol.HandleConn(client.conn, sc.handleMessage)
	}()
	// send start sync cmd
	resp, err := client.Send(protocol.CtrlSyncStart, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "receive sync start response failed")
	}
	if !bytes.Equal(resp, []byte{protocol.CtrlSyncStart}) {
		err = errors.WithMessage(err, "invalid sync start response")
		sc.log(logger.EXPLOIT, err)
		return nil, err
	}
	return &sc, nil
}

func (sc *sClient) Broadcast(token, message []byte) *protocol.BroadcastResponse {
	br := protocol.BroadcastResponse{}
	br.Role = protocol.Node
	br.GUID = sc.guid
	reply, err := sc.client.Send(protocol.CtrlBroadcastToken, token)
	if err != nil {
		br.Err = err
		return &br
	}
	if !bytes.Equal(reply, protocol.BroadcastUnhandled) {
		br.Err = protocol.ErrBroadcastHandled
		return &br
	}
	// broadcast
	reply, err = sc.client.Send(protocol.CtrlBroadcast, message)
	if err != nil {
		br.Err = err
		return &br
	}
	if bytes.Equal(reply, protocol.BroadcastSucceed) {
		return &br
	} else {
		br.Err = errors.New(string(reply))
		return &br
	}
}

func (sc *sClient) SyncSend(token, message []byte) *protocol.SyncResponse {
	sr := &protocol.SyncResponse{}
	sr.Role = protocol.Node
	sr.GUID = sc.guid
	resp, err := sc.client.Send(protocol.CtrlSyncSendToken, token)
	if err != nil {
		sr.Err = err
		return sr
	}
	if !bytes.Equal(resp, protocol.SyncUnhandled) {
		sr.Err = protocol.ErrSyncHandled
		return sr
	}
	resp, err = sc.client.Send(protocol.CtrlSyncSend, message)
	if err != nil {
		sr.Err = err
		return sr
	}
	if bytes.Equal(resp, protocol.SyncSucceed) {
		return sr
	} else {
		sr.Err = errors.New(string(resp))
		return sr
	}
}

// notice node clean the message
func (sc *sClient) SyncReceive(token, message []byte) *protocol.SyncResponse {
	sr := &protocol.SyncResponse{}
	sr.Role = protocol.Node
	sr.GUID = sc.guid
	resp, err := sc.client.Send(protocol.CtrlSyncRecvToken, token)
	if err != nil {
		sr.Err = err
		return sr
	}
	if !bytes.Equal(resp, protocol.SyncUnhandled) {
		sr.Err = protocol.ErrSyncHandled
		return sr
	}
	resp, err = sc.client.Send(protocol.CtrlSyncRecv, message)
	if err != nil {
		sr.Err = err
		return sr
	}
	if bytes.Equal(resp, protocol.SyncSucceed) {
		return sr
	} else {
		sr.Err = errors.New(string(resp))
		return sr
	}
}

func (sc *sClient) QueryMessage(request []byte) (*protocol.SyncReply, error) {
	reply, err := sc.client.Send(protocol.CtrlSyncQuery, request)
	if err != nil {
		return nil, err
	}
	sr := protocol.SyncReply{}
	err = msgpack.Unmarshal(reply, &sr)
	if err != nil {
		err = errors.Wrap(err, "invalid sync reply")
		sc.log(logger.EXPLOIT, err)
		sc.Close()
		return nil, err
	}
	return &sr, nil
}

func (sc *sClient) Close() {
	sc.client.Close()
}

func (sc *sClient) logf(l logger.Level, format string, log ...interface{}) {
	b := logger.Conn(sc.client.conn)
	_, _ = fmt.Fprintf(b, format, log...)
	sc.ctx.ctx.Print(l, "syncer-client", b)
}

func (sc *sClient) log(l logger.Level, log ...interface{}) {
	b := logger.Conn(sc.client.conn)
	_, _ = fmt.Fprint(b, log...)
	sc.ctx.ctx.Print(l, "syncer-client", b)
}

func (sc *sClient) logln(l logger.Level, log ...interface{}) {
	b := logger.Conn(sc.client.conn)
	_, _ = fmt.Fprintln(b, log...)
	sc.ctx.ctx.Print(l, "syncer-client", b)
}

// can use client.Close()
func (sc *sClient) handleMessage(msg []byte) {
	const (
		cmd = protocol.MsgCMDSize
		id  = protocol.MsgCMDSize + protocol.MsgIDSize
	)
	if sc.client.isClosed() {
		return
	}
	// cmd(1) + msg id(2) or reply
	if len(msg) < id {
		sc.log(logger.EXPLOIT, protocol.ErrInvalidMsgSize)
		sc.Close()
		return
	}
	switch msg[0] {
	case protocol.NodeSyncSendToken:
		sc.handleSyncSendToken(msg[cmd:id], msg[id:])
	case protocol.NodeSyncSend:
		sc.handleSyncSend(msg[cmd:id], msg[id:])
	case protocol.NodeSyncRecvToken:
		sc.handleSyncReceiveToken(msg[cmd:id], msg[id:])
	case protocol.NodeSyncRecv:
		sc.handleSyncReceive(msg[cmd:id], msg[id:])
	case protocol.NodeBroadcastToken:
		sc.handleBroadcastToken(msg[cmd:id], msg[id:])
	case protocol.NodeBroadcast:
		sc.handleBroadcast(msg[cmd:id], msg[id:])
	// ---------------------------internal--------------------------------
	case protocol.NodeReply:
		sc.client.handleReply(msg[cmd:])
	case protocol.NodeHeartbeat:
		sc.client.heartbeatC <- struct{}{}
	case protocol.ErrNullMsg:
		sc.log(logger.EXPLOIT, protocol.ErrRecvNullMsg)
		sc.Close()
	case protocol.ErrTooBigMsg:
		sc.log(logger.EXPLOIT, protocol.ErrRecvTooBigMsg)
		sc.Close()
	case protocol.TestMessage:
		sc.client.Reply(msg[cmd:id], msg[id:])
	default:
		sc.log(logger.EXPLOIT, protocol.ErrRecvUnknownCMD, msg)
		sc.Close()
		return
	}
}

func (sc *sClient) handleBroadcastToken(id, message []byte) {
	// role + message guid
	if len(message) != 1+guid.SIZE {
		// fake reply and close
		sc.client.Reply(id, protocol.BroadcastHandled)
		sc.log(logger.EXPLOIT, "invalid broadcast token size")
		sc.Close()
		return
	}
	if sc.ctx.checkBroadcastToken(message[0], message[1:1+guid.SIZE]) {
		sc.client.Reply(id, protocol.BroadcastUnhandled)
	} else {
		sc.client.Reply(id, protocol.BroadcastHandled)
	}
}

func (sc *sClient) handleSyncSendToken(id, message []byte) {
	if len(message) != 1+guid.SIZE {
		// fake reply and close
		sc.client.Reply(id, protocol.SyncHandled)
		sc.log(logger.EXPLOIT, "invalid sync send token size")
		sc.Close()
		return
	}
	if sc.ctx.checkSyncSendToken(message[0], message[1:1+guid.SIZE]) {
		sc.client.Reply(id, protocol.SyncUnhandled)
	} else {
		sc.client.Reply(id, protocol.SyncHandled)
	}
}

func (sc *sClient) handleSyncReceiveToken(id, message []byte) {
	if len(message) != 1+guid.SIZE {
		// fake reply and close
		sc.client.Reply(id, protocol.SyncHandled)
		sc.log(logger.EXPLOIT, "invalid sync receive token size")
		sc.Close()
		return
	}
	if sc.ctx.checkSyncReceiveToken(message[0], message[1:1+guid.SIZE]) {
		sc.client.Reply(id, protocol.SyncUnhandled)
	} else {
		sc.client.Reply(id, protocol.SyncHandled)
	}
}

func (sc *sClient) handleBroadcast(id, message []byte) {
	br := protocol.Broadcast{}
	err := msgpack.Unmarshal(message, &br)
	if err != nil {
		sc.log(logger.EXPLOIT, "invalid broadcast msgpack data")
		sc.Close()
		return
	}
	err = br.Validate()
	if err != nil {
		sc.log(logger.EXPLOIT, "invalid broadcast", err)
		sc.Close()
		return
	}
	sc.ctx.addBroadcast(&br)
	sc.client.Reply(id, protocol.BroadcastSucceed)
}

// Node -> Controller(direct)
func (sc *sClient) handleSyncSend(id, message []byte) {
	ss := protocol.SyncSend{}
	err := msgpack.Unmarshal(message, &ss)
	if err != nil {
		sc.log(logger.EXPLOIT, "invalid sync send msgpack data")
		sc.Close()
		return
	}
	err = ss.Validate()
	if err != nil {
		sc.log(logger.EXPLOIT, "invalid sync send", err)
		sc.Close()
		return
	}
	sc.ctx.addSyncSend(&ss)
	sc.client.Reply(id, protocol.SyncSucceed)
}

// notice controller role received this height message
func (sc *sClient) handleSyncReceive(id, message []byte) {
	sr := protocol.SyncReceive{}
	err := msgpack.Unmarshal(message, &sr)
	if err != nil {
		sc.log(logger.EXPLOIT, "invalid sync receive msgpack data")
		sc.Close()
		return
	}
	err = sr.Validate()
	if err != nil {
		sc.log(logger.EXPLOIT, "invalid sync receive:", err)
		sc.Close()
		return
	}
	sc.ctx.addSyncReceive(&sr)
	sc.client.Reply(id, protocol.SyncSucceed)
}
