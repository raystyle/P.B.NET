package node

import (
	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/protocol"
)

type workerManager struct {
	ctx *Node

	// from controller
	broadcastQueue chan *protocol.Broadcast
	sendQueue      chan *protocol.Send
}

func newWorkerManager(ctx *Node, config *Config) (*workerManager, error) {
	cfg := config.Worker
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

	// start workers
	for i := 0; i < cfg.Worker; i++ {
		syncer.wg.Add(1)
		go syncer.worker()
	}

}

func (syncer *syncer) logf(l logger.Level, format string, log ...interface{}) {
	syncer.ctx.Printf(l, "syncer", format, log...)
}

func (syncer *syncer) log(l logger.Level, log ...interface{}) {
	syncer.ctx.Print(l, "syncer", log...)
}

func (syncer *syncer) logln(l logger.Level, log ...interface{}) {
	syncer.ctx.Println(l, "syncer", log...)
}

type worker struct {
}
