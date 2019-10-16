package controller

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"hash"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type syncer struct {
	ctx *CTRL

	expireTime float64

	// key = base64(GUID) value = timestamp
	nodeSendGUID       map[string]int64
	nodeSendGUIDRWM    sync.RWMutex
	beaconSendGUID     map[string]int64
	beaconSendGUIDRWM  sync.RWMutex
	beaconQueryGUID    map[string]int64
	beaconQueryGUIDRWM sync.RWMutex

	nodeSendQueue    chan *protocol.Send
	beaconSendQueue  chan *protocol.Send
	beaconQueryQueue chan *protocol.Query

	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newSyncer(ctx *CTRL, config *Config) (*syncer, error) {
	cfg := config.Syncer
	// check config
	if cfg.MaxBufferSize < 4096 {
		return nil, errors.New("max buffer size < 4096")
	}
	if cfg.Worker < 2 {
		return nil, errors.New("worker number < 2")
	}
	if cfg.QueueSize < 512 {
		return nil, errors.New("worker task queue size < 512")
	}
	if cfg.ExpireTime < 5*time.Minute || cfg.ExpireTime > time.Hour {
		return nil, errors.New("expire time < 5m or > 1h")
	}
	syncer := syncer{
		ctx:              ctx,
		expireTime:       cfg.ExpireTime.Seconds(),
		nodeSendGUID:     make(map[string]int64, cfg.QueueSize),
		beaconSendGUID:   make(map[string]int64, cfg.QueueSize),
		beaconQueryGUID:  make(map[string]int64, cfg.QueueSize),
		nodeSendQueue:    make(chan *protocol.Send, cfg.QueueSize),
		beaconSendQueue:  make(chan *protocol.Send, cfg.QueueSize),
		beaconQueryQueue: make(chan *protocol.Query, cfg.QueueSize),
		stopSignal:       make(chan struct{}),
	}
	// start workers
	for i := 0; i < cfg.Worker; i++ {
		worker := syncerWorker{
			ctx:           &syncer,
			maxBufferSize: cfg.MaxBufferSize,
		}
		syncer.wg.Add(1)
		go worker.Work()
	}
	syncer.wg.Add(1)
	go syncer.guidCleaner()
	return &syncer, nil
}

func (syncer *syncer) Close() {
	close(syncer.stopSignal)
	syncer.wg.Wait()
}

func (syncer *syncer) logf(l logger.Level, format string, log ...interface{}) {
	syncer.ctx.logger.Printf(l, "syncer", format, log...)
}

func (syncer *syncer) log(l logger.Level, log ...interface{}) {
	syncer.ctx.logger.Print(l, "syncer", log...)
}

func (syncer *syncer) logln(l logger.Level, log ...interface{}) {
	syncer.ctx.logger.Println(l, "syncer", log...)
}

// task from syncer client
func (syncer *syncer) AddNodeSend(s *protocol.Send) {
	select {
	case syncer.nodeSendQueue <- s:
	case <-syncer.stopSignal:
	}
}

func (syncer *syncer) AddBeaconSend(s *protocol.Send) {
	select {
	case syncer.beaconSendQueue <- s:
	case <-syncer.stopSignal:
	}
}

func (syncer *syncer) AddBeaconQuery(q *protocol.Query) {
	select {
	case syncer.beaconQueryQueue <- q:
	case <-syncer.stopSignal:
	}
}

func (syncer *syncer) CheckGUIDTimestamp(guid []byte) (bool, int64) {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.expireTime {
		return false, 0
	}
	return true, timestamp
}

func (syncer *syncer) CheckNodeSendGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.nodeSendGUIDRWM.Lock()
		if _, ok := syncer.nodeSendGUID[key]; ok {
			syncer.nodeSendGUIDRWM.Unlock()
			return false
		} else {
			syncer.nodeSendGUID[key] = timestamp
			syncer.nodeSendGUIDRWM.Unlock()
			return true
		}
	} else {
		syncer.nodeSendGUIDRWM.RLock()
		_, ok := syncer.nodeSendGUID[key]
		syncer.nodeSendGUIDRWM.RUnlock()
		return !ok
	}
}

func (syncer *syncer) CheckBeaconSendGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.beaconSendGUIDRWM.Lock()
		if _, ok := syncer.beaconSendGUID[key]; ok {
			syncer.beaconSendGUIDRWM.Unlock()
			return false
		} else {
			syncer.beaconSendGUID[key] = timestamp
			syncer.beaconSendGUIDRWM.Unlock()
			return true
		}
	} else {
		syncer.beaconSendGUIDRWM.RLock()
		_, ok := syncer.beaconSendGUID[key]
		syncer.beaconSendGUIDRWM.RUnlock()
		return !ok
	}
}

func (syncer *syncer) CheckBeaconQueryGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.beaconQueryGUIDRWM.Lock()
		if _, ok := syncer.beaconQueryGUID[key]; ok {
			syncer.beaconQueryGUIDRWM.Unlock()
			return false
		} else {
			syncer.beaconQueryGUID[key] = timestamp
			syncer.beaconQueryGUIDRWM.Unlock()
			return true
		}
	} else {
		syncer.beaconQueryGUIDRWM.RLock()
		_, ok := syncer.beaconQueryGUID[key]
		syncer.beaconQueryGUIDRWM.RUnlock()
		return !ok
	}
}

// guidCleaner is use to clean expired guid
func (syncer *syncer) guidCleaner() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("syncer guid cleaner panic:", r)
			syncer.log(logger.Fatal, err)
			// restart guid cleaner
			time.Sleep(time.Second)
			go syncer.guidCleaner()
		} else {
			syncer.wg.Done()
		}
	}()
	ticker := time.NewTicker(time.Duration(syncer.expireTime / 100))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := syncer.ctx.global.Now().Unix()
			// clean node send
			syncer.nodeSendGUIDRWM.Lock()
			for key, timestamp := range syncer.nodeSendGUID {
				if float64(now-timestamp) > syncer.expireTime {
					delete(syncer.nodeSendGUID, key)
				}
			}
			syncer.nodeSendGUIDRWM.Unlock()
			// clean beacon send
			syncer.beaconSendGUIDRWM.Lock()
			for key, timestamp := range syncer.beaconSendGUID {
				if float64(now-timestamp) > syncer.expireTime {
					delete(syncer.beaconSendGUID, key)
				}
			}
			syncer.beaconSendGUIDRWM.Unlock()
			// clean beacon query
			syncer.beaconQueryGUIDRWM.Lock()
			for key, timestamp := range syncer.beaconQueryGUID {
				if float64(now-timestamp) > syncer.expireTime {
					delete(syncer.beaconQueryGUID, key)
				}
			}
			syncer.beaconQueryGUIDRWM.Unlock()
		case <-syncer.stopSignal:
			return
		}
	}
}

type syncerWorker struct {
	ctx           *syncer
	maxBufferSize int

	// task
	send  *protocol.Send
	query *protocol.Query

	// key
	node      *mNode
	beacon    *mBeacon
	publicKey ed25519.PublicKey
	aesKey    []byte
	aesIV     []byte

	buffer         *bytes.Buffer
	msgpackEncoder *msgpack.Encoder
	hash           hash.Hash

	err error
}

func (sw *syncerWorker) Work() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("syncerWorker.Work() panic:", r)
			sw.ctx.log(logger.Fatal, err)
			// restart worker
			time.Sleep(time.Second)
			go sw.Work()
		} else {
			sw.ctx.wg.Done()
		}
	}()
	// init buffer
	// protocol.SyncReceive buffer cap = guid.Size + 8 + 1 + guid.Size
	minBufferSize := 2*guid.Size + 9
	sw.buffer = bytes.NewBuffer(make([]byte, minBufferSize))
	sw.hash = sha256.New()
	// start handle task
	for {
		// check buffer capacity
		if sw.buffer.Cap() > sw.maxBufferSize {
			sw.buffer = bytes.NewBuffer(make([]byte, minBufferSize))
		}
		select {
		case sw.query = <-sw.ctx.beaconQueryQueue:
			sw.handleBeaconQuery()
		case sw.send = <-sw.ctx.beaconSendQueue:
			sw.handleBeaconSend()
		case sw.send = <-sw.ctx.nodeSendQueue:
			sw.handleNodeSend()
		case <-sw.ctx.stopSignal:
			return
		}
	}
}

func (sw *syncerWorker) handleBeaconQuery() {
	// set key
	sw.beacon, sw.err = sw.ctx.ctx.db.SelectBeacon(sw.query.BeaconGUID)
	if sw.err != nil {
		sw.ctx.logf(logger.Warning, "select beacon %X failed %s", sw.query.BeaconGUID, sw.err)
		return
	}
	sw.publicKey = sw.beacon.PublicKey
	sw.aesKey = sw.beacon.SessionKey
	sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(sw.query.GUID)
	sw.buffer.Write(sw.query.BeaconGUID)
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), sw.query.Signature) {
		sw.ctx.logf(logger.Exploit, "invalid query signature guid: %X", sw.query.BeaconGUID)
		return
	}
	// TODO broadcast message
}

func (sw *syncerWorker) handleBeaconSend() {
	// set key
	sw.beacon, sw.err = sw.ctx.ctx.db.SelectBeacon(sw.send.RoleGUID)
	if sw.err != nil {
		sw.ctx.logf(logger.Warning, "select beacon %X failed %s", sw.send.RoleGUID, sw.err)
		return
	}
	sw.publicKey = sw.beacon.PublicKey
	sw.aesKey = sw.beacon.SessionKey
	sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(sw.send.GUID)
	sw.buffer.Write(sw.send.RoleGUID)
	sw.buffer.Write(sw.send.Message)
	sw.buffer.Write(sw.send.Hash)
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), sw.send.Signature) {
		sw.ctx.logf(logger.Exploit, "invalid beacon send signature guid: %X", sw.send.RoleGUID)
		return
	}
	// decrypt
	sw.send.Message, sw.err = aes.CBCDecrypt(sw.send.Message, sw.aesKey, sw.aesIV)
	if sw.err != nil {
		sw.ctx.logf(logger.Exploit, "decrypt beacon %X send failed: %s", sw.send.RoleGUID, sw.err)
		return
	}
	// check hash
	sw.hash.Reset()
	sw.hash.Write(sw.send.Message)
	if subtle.ConstantTimeCompare(sw.hash.Sum(nil), sw.send.Hash) != 1 {
		sw.ctx.logf(logger.Exploit, "beacon %X send with wrong hash", sw.send.RoleGUID)
		return
	}
	sw.ctx.ctx.handler.HandleBeaconSend(sw.send)
	sw.ctx.ctx.sender.Acknowledge(protocol.Beacon, sw.send)
}

func (sw *syncerWorker) handleNodeSend() {
	// set key
	sw.node, sw.err = sw.ctx.ctx.db.SelectNode(sw.send.RoleGUID)
	if sw.err != nil {
		sw.ctx.logf(logger.Warning, "select node %X failed %s", sw.send.RoleGUID, sw.err)
		return
	}
	sw.publicKey = sw.node.PublicKey
	sw.aesKey = sw.node.SessionKey
	sw.aesIV = sw.node.SessionKey[:aes.IVSize]
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(sw.send.GUID)
	sw.buffer.Write(sw.send.RoleGUID)
	sw.buffer.Write(sw.send.Message)
	sw.buffer.Write(sw.send.Hash)
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), sw.send.Signature) {
		sw.ctx.logf(logger.Exploit, "invalid node send signature guid: %X", sw.send.RoleGUID)
		return
	}
	// decrypt
	sw.send.Message, sw.err = aes.CBCDecrypt(sw.send.Message, sw.aesKey, sw.aesIV)
	if sw.err != nil {
		sw.ctx.logf(logger.Exploit, "decrypt node %X send failed: %s", sw.send.RoleGUID, sw.err)
		return
	}
	// check hash
	sw.hash.Reset()
	sw.hash.Write(sw.send.Message)
	if subtle.ConstantTimeCompare(sw.hash.Sum(nil), sw.send.Hash) != 1 {
		sw.ctx.logf(logger.Exploit, "node %X send with wrong hash", sw.send.RoleGUID)
		return
	}
	sw.ctx.ctx.handler.HandleNodeSend(sw.send)
	sw.ctx.ctx.sender.Acknowledge(protocol.Node, sw.send)
}
