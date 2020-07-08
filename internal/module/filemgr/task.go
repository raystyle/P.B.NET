package filemgr

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/looplab/fsm"
	"github.com/pkg/errors"
)

const (
	taskTypeCopy       = "copy"
	taskTypeMove       = "move"
	taskTypeCompress   = "compress"
	taskTypeDecompress = "decompress"
)

type taskConfig struct {
	callbacks fsm.Callbacks
	collect   func(ctx context.Context) error
	process   func(ctx context.Context, checkPaused func()) error
	progress  func() float32
	detail    func() string
}

// Task is a task that contains all information about special task.
type Task struct {
	typ string
	cfg *taskConfig
	fsm *fsm.FSM

	// about pause task and control
	paused   *int32        // 0 = processing, 1 = paused, 2 = canceled
	pausedCh chan struct{} // prevent paused chan block
	finished bool          // prevent cancel twice
	mu       sync.Mutex

	startOnce  sync.Once
	cancelOnce sync.Once
	ctx        context.Context
	cancel     context.CancelFunc
}

// newTask is used to create a task with callbacks, ctx can cancel task.
func newTask(ctx context.Context, typ string, cfg *taskConfig) (*Task, error) {
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
	FSM := fsm.NewFSM(StateReady, events, cfg.callbacks)
	// create task
	task := Task{
		typ:      typ,
		cfg:      cfg,
		fsm:      FSM,
		paused:   new(int32),
		pausedCh: make(chan struct{}, 1),
	}
	task.ctx, task.cancel = context.WithCancel(ctx)
	return &task, nil
}

// Start is used to start current task.
func (task *Task) Start() (err error) {
	task.startOnce.Do(func() {
		if !task.start() {
			err = errors.New("task canceled")
			return
		}
		err = task.collect()
		if err != nil {
			return
		}
		if !task.checkProcess() {
			err = errors.New("task canceled")
			return
		}
		err = task.process()
	})
	return
}

func (task *Task) start() bool {
	task.mu.Lock()
	defer task.mu.Unlock()
	if task.finished {
		return false
	}
	err := task.fsm.Event(EventStart)
	if err != nil {
		panic(fmt.Sprintf("filemgr: internal error: %s", err))
	}
	return true
}

func (task *Task) collect() error {
	err := task.cfg.collect(task.ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to collect")
	}
	return nil
}

func (task *Task) checkProcess() bool {
	task.mu.Lock()
	defer task.mu.Unlock()
	if task.finished {
		return false
	}
	err := task.fsm.Event(EventProcess)
	if err != nil {
		panic(fmt.Sprintf("filemgr: internal error: %s", err))
	}
	return true
}

func (task *Task) process() error {
	err := task.cfg.process(task.ctx, task.checkPaused)
	if err != nil {
		return err
	}
	task.mu.Lock()
	defer task.mu.Unlock()
	if task.finished {
		return errors.New("task canceled")
	}
	task.finished = true
	err = task.fsm.Event(EventComplete)
	if err != nil {
		panic(fmt.Sprintf("filemgr: internal error: %s", err))
	}
	return nil
}

// checkPaused is used to check current task is paused in process function.
func (task *Task) checkPaused() {
	if atomic.LoadInt32(task.paused) != 1 {
		return
	}
	select {
	case <-task.pausedCh:
	case <-task.ctx.Done():
	}
}

// Pause is used to pause current progress(collect or process).
func (task *Task) Pause() error {
	if !atomic.CompareAndSwapInt32(task.paused, 0, 1) {
		return nil
	}
	task.mu.Lock()
	defer task.mu.Unlock()
	if task.finished {
		return nil
	}
	return task.fsm.Event(EventPause)
}

// Continue is used to continue current task.
func (task *Task) Continue() error {
	if !atomic.CompareAndSwapInt32(task.paused, 1, 0) {
		return nil
	}
	task.mu.Lock()
	defer task.mu.Unlock()
	if task.finished {
		return nil
	}
	select {
	case task.pausedCh <- struct{}{}:
		return task.fsm.Event(EventContinue)
	default:
		return nil
	}
}

// Cancel is used to cancel current task.
func (task *Task) Cancel() {
	task.cancelOnce.Do(func() {
		atomic.StoreInt32(task.paused, 2)

		task.mu.Lock()
		defer task.mu.Unlock()
		if task.finished {
			return
		}
		close(task.pausedCh)
		task.cancel()
		task.finished = true

		err := task.fsm.Event(EventCancel)
		if err != nil {
			panic(fmt.Sprintf("filemgr: internal error: %s", err))
		}
	})
}

// Type is used to get the type of current task.
func (task *Task) Type() string {
	return task.typ
}

// State is used to get the state about current task.
func (task *Task) State() string {
	return task.fsm.Current()
}

// Progress is used to get the progress about current task.
func (task *Task) Progress() float32 {
	return task.cfg.progress()
}

// Detail is used to get the detail about current task.
func (task *Task) Detail() string {
	return task.cfg.detail()
}
