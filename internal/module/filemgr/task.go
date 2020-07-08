package filemgr

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/looplab/fsm"
)

// CopyTask is a task that contains all information about copy.
type CopyTask struct {
	ec    ErrCtrl
	stats *srcDstStat
	fsm   *fsm.FSM

	// about pause task and control
	paused    *int32        // 0 = processing, 1 = paused, 2 = canceled
	pausedCh  chan struct{} // prevent paused chan block
	completed bool          // prevent cancel twice

	mu         sync.Mutex
	startOnce  sync.Once
	cancelOnce sync.Once
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewCopyTask is used to create a copy task, ctx can cancel task.
func NewCopyTask(ctx context.Context, ec ErrCtrl, src, dst string) (*CopyTask, error) {
	return NewCopyTaskWithCallBacks(ctx, ec, nil, src, dst)
}

// NewCopyTaskWithCallBacks is used to create a copy task with callbacks, ctx can cancel task.
func NewCopyTaskWithCallBacks(
	ctx context.Context,
	ec ErrCtrl,
	callbacks fsm.Callbacks,
	src string,
	dst string,
) (*CopyTask, error) {
	stats, err := checkSrcDstPath(src, dst)
	if err != nil {
		return nil, err
	}
	// initial FSM
	cancelEvent := fsm.EventDesc{
		Name: EventCancel,
		Src:  []string{StateReady, StateCollect, StateProcess, StatePause},
		Dst:  StateCancel,
	}
	events := []fsm.EventDesc{
		{EventStart, []string{StateReady}, StateCollect},
		{EventProcess, []string{StateCollect}, StateProcess},
		{EventPause, []string{StateProcess}, StatePause},
		{EventContinue, []string{StatePause}, StateProcess},
		{EventComplete, []string{StateProcess}, StateComplete},
		cancelEvent,
	}
	FSM := fsm.NewFSM(StateReady, events, callbacks)

	ct := CopyTask{
		ec:       ec,
		stats:    stats,
		fsm:      FSM,
		paused:   new(int32),
		pausedCh: make(chan struct{}, 1),
	}
	ct.ctx, ct.cancel = context.WithCancel(ctx)
	return &ct, nil
}

// Start is used to start this copy task.
func (ct *CopyTask) Start() (err error) {
	ct.startOnce.Do(func() {
		err = ct.start()

	})
	return
}

func (ct *CopyTask) start() error {

	return nil
}

func (ct *CopyTask) collect() {

}

func (ct *CopyTask) process() {

	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.completed = true
}

// io copy loop will call it each copy.
func (ct *CopyTask) checkPaused() {
	if atomic.LoadInt32(ct.paused) == 1 {
		select {
		case <-ct.pausedCh:
		case <-ct.ctx.Done():
		}
	}
}

// Pause is used to pause current progress(collect or process).
func (ct *CopyTask) Pause() error {
	if atomic.CompareAndSwapInt32(ct.paused, 0, 1) {
		return ct.fsm.Event(EventPause)
	}
	return nil
}

// Continue is used to continue current task.
func (ct *CopyTask) Continue() error {
	if atomic.CompareAndSwapInt32(ct.paused, 1, 0) {
		ct.mu.Lock()
		defer ct.mu.Unlock()
		if ct.completed {
			return nil
		}
		select {
		case ct.pausedCh <- struct{}{}:
			return ct.fsm.Event(EventContinue)
		default:
		}
	}
	return nil
}

// Cancel is used to cancel current copy task.
func (ct *CopyTask) Cancel() {
	ct.cancelOnce.Do(func() {
		atomic.StoreInt32(ct.paused, 2)

		ct.mu.Lock()
		defer ct.mu.Unlock()
		if ct.completed {
			return
		}
		close(ct.pausedCh)
		ct.cancel()
		ct.completed = true

		err := ct.fsm.Event(EventCancel)
		if err != nil {
			panic(fmt.Sprintf("filemgr: internal error: %s", err))
		}
	})
}
