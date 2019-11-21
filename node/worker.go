package node

import (
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type workerManager struct {
	broadcastQueue chan *protocol.Broadcast
	sendQueue      chan *protocol.Send

	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newWorkerManager(ctx *Node, config *Config) (*workerManager, error) {
	cfg := config.Worker
	// check config
	if cfg.Number < 4 {
		return nil, errors.New("worker number must >= 4")
	}
	if cfg.QueueSize < 512 {
		return nil, errors.New("worker task queue size < 512")
	}
	if cfg.MaxBufferSize < 4096 {
		return nil, errors.New("max buffer size >= 4096")
	}

	manager := workerManager{
		broadcastQueue: make(chan *protocol.Broadcast),
		sendQueue:      make(chan *protocol.Send),
		stopSignal:     make(chan struct{}),
	}

	// start workers
	for i := 0; i < cfg.Number; i++ {
		worker := worker{
			ctx:            ctx,
			maxBufferSize:  cfg.MaxBufferSize,
			broadcastQueue: manager.broadcastQueue,
			sendQueue:      manager.sendQueue,
			stopSignal:     manager.stopSignal,
			wg:             &manager.wg,
		}
		manager.wg.Add(1)
		go worker.Work()
	}
	return &manager, nil
}

func (wm *workerManager) Close() {
	close(wm.stopSignal)
}

type worker struct {
	ctx *Node

	maxBufferSize int

	broadcastQueue chan *protocol.Broadcast
	sendQueue      chan *protocol.Send

	stopSignal chan struct{}
	wg         *sync.WaitGroup
}

func (worker *worker) logf(l logger.Level, format string, log ...interface{}) {
	worker.ctx.logger.Printf(l, "worker", format, log...)
}

func (worker *worker) log(l logger.Level, log ...interface{}) {
	worker.ctx.logger.Print(l, "worker", log...)
}

func (worker *worker) Work() {
	defer func() {
		if r := recover(); r != nil {
			worker.log(logger.Fatal, xpanic.Error(r, "worker.Work()"))
			// restart worker
			time.Sleep(time.Second)
			go worker.Work()
		} else {
			worker.wg.Done()
		}
	}()

}
