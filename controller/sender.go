package controller

import (
	"sync"

	"github.com/pkg/errors"

	"project/internal/guid"
	"project/internal/protocol"
)

const (
	senderNode   = 0
	senderBeacon = 1
)

type broadcastTask struct {
	Role     protocol.Role
	Command  []byte      // for Broadcast
	MessageI interface{} // for Broadcast
	Message  []byte      // for BroadcastPlugin
	Result   chan<- *protocol.BroadcastResult
}

type syncSendTask struct {
	Role     protocol.Role
	Target   []byte
	Command  []byte      // for Send
	MessageI interface{} // for Send
	Message  []byte      // for SendPlugin
	Result   chan<- *protocol.SyncResult
}

type syncReceiveTask struct {
	Role   protocol.Role
	GUID   []byte
	Height uint64
}

type sender struct {
	bufferSize       int
	broadcastQueue   chan *broadcastTask
	syncSendQueue    chan *syncSendTask
	syncReceiveQueue chan *syncReceiveTask
	// role can be only one sync at th same time
	// key=base64(sender guid) 0=node 1=beacon
	syncSendMs  [2]map[string]*sync.Mutex
	syncSendRWM [2]sync.RWMutex
	guid        *guid.GUID
	stopSignal  chan struct{}
	wg          sync.WaitGroup
}

func newSender(ctx *CTRL, cfg *Config) (*sender, error) {
	if cfg.BufferSize < 4096 {
		return nil, errors.New("buffer size < 4096")
	}
	if cfg.SenderNumber < 1 {
		return nil, errors.New("sender number < 1")
	}
	if cfg.SenderQueueSize < 512 {
		return nil, errors.New("sender queue size < 512")
	}
	sender := sender{
		bufferSize:       cfg.BufferSize,
		broadcastQueue:   make(chan *broadcastTask, cfg.SenderQueueSize),
		syncSendQueue:    make(chan *syncSendTask, cfg.SenderQueueSize),
		syncReceiveQueue: make(chan *syncReceiveTask, cfg.SenderQueueSize),
		guid:             guid.New(512*cfg.SenderNumber, ctx.global.Now),
		stopSignal:       make(chan struct{}),
	}
	sender.syncSendMs[senderNode] = make(map[string]*sync.Mutex)
	sender.syncSendMs[senderBeacon] = make(map[string]*sync.Mutex)
	return &sender, nil
}

func (sender *sender) Broadcast(
	role protocol.Role,
	command []byte,
	message interface{},
) *protocol.BroadcastResult {
	done := make(chan *protocol.BroadcastResult, 1)
	sender.broadcastQueue <- &broadcastTask{
		Role:     role,
		Command:  command,
		MessageI: message,
		Result:   done,
	}
	return <-done
}

func (sender *sender) BroadcastAsync(
	role protocol.Role,
	command []byte,
	message interface{},
	done chan<- *protocol.BroadcastResult,
) {
	sender.broadcastQueue <- &broadcastTask{
		Role:     role,
		Command:  command,
		MessageI: message,
		Result:   done,
	}
}

func (sender *sender) BroadcastPlugin(
	role protocol.Role,
	message []byte,
) *protocol.BroadcastResult {
	done := make(chan *protocol.BroadcastResult, 1)
	sender.broadcastQueue <- &broadcastTask{
		Role:    role,
		Message: message,
		Result:  done,
	}
	return <-done
}

func (sender *sender) Send(
	role protocol.Role,
	target,
	command []byte,
	message interface{},
) *protocol.SyncResult {
	done := make(chan *protocol.SyncResult, 1)
	sender.syncSendQueue <- &syncSendTask{
		Role:     role,
		Target:   target,
		Command:  command,
		MessageI: message,
		Result:   done,
	}
	return <-done
}

func (sender *sender) SendAsync(
	role protocol.Role,
	target,
	command []byte,
	message interface{},
	done chan<- *protocol.SyncResult,
) {
	sender.syncSendQueue <- &syncSendTask{
		Role:     role,
		Target:   target,
		Command:  command,
		MessageI: message,
		Result:   done,
	}
}

func (sender *sender) SendPlugin(
	role protocol.Role,
	target,
	message []byte,
	done chan<- *protocol.SyncResult,
) {
	sender.syncSendQueue <- &syncSendTask{
		Role:    role,
		Target:  target,
		Message: message,
		Result:  done,
	}
}

func (sender *sender) sender() {

}
