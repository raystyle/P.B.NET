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
		return nil, errors.New("max buffer size >= 4096")
	}
	if cfg.Worker < 4 {
		return nil, errors.New("worker number must >= 4")
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

// guidCleaner is use to clean expired guid
func (syncer *syncer) guidCleaner() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "syncer guid cleaner panic:")
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
