package task

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/looplab/fsm"
	"github.com/pkg/errors"

	"project/internal/xpanic"
)

// states about task
const (
	StateReady    = "ready"    // wait call Start()
	StatePrepare  = "prepare"  // prepare task like walk directory.
	StateProcess  = "process"  // doing task
	StatePause    = "pause"    // appear error or user pause progress
	StateComplete = "complete" // task finished
	StateCancel   = "cancel"   // task canceled
)

// events about task
const (
	EventStart    = "start"    // task start
	EventProcess  = "process"  // update progress
	EventPause    = "pause"    // pause update process progress
	EventContinue = "continue" // continue update process progress
	EventComplete = "complete" // task completed
	EventCancel   = "cancel"   // task canceled not update progress
)

const panicFormat = "task: internal error: %s"

// Interface is the interface about task.
type Interface interface {
	Prepare(ctx context.Context) error
	Process(ctx context.Context, task *Task) error
	Clean()
	Progress() float32
	Detail() string
}

// Task is a task that contains all information about special task.
// It provide Pause task and get task progress.
type Task struct {
	name string
	task Interface
	fsm  *fsm.FSM

	// about pause task and control
	paused   *int32        // 0 = processing, 1 = paused, 2 = canceled
	pausedCh chan struct{} // prevent paused chan block
	finished bool          // prevent cancel twice
	mu       sync.Mutex

	startOnce  sync.Once
	cancelOnce sync.Once
	cleanOnce  sync.Once
	ctx        context.Context
	cancel     context.CancelFunc
}

// New is used to create a task with callbacks, ctx can cancel task.
func New(name string, iface Interface, callbacks fsm.Callbacks) *Task {
	// initial FSM
	cancelEvent := fsm.EventDesc{
		Name: EventCancel,
		Src:  []string{StateReady, StatePrepare, StateProcess, StatePause},
		Dst:  StateCancel,
	}
	events := []fsm.EventDesc{
		{EventStart, []string{StateReady}, StatePrepare},
		{EventProcess, []string{StatePrepare}, StateProcess},
		{EventPause, []string{StateProcess}, StatePause},
		{EventContinue, []string{StatePause}, StateProcess},
		{EventComplete, []string{StateProcess}, StateComplete},
		cancelEvent,
	}
	FSM := fsm.NewFSM(StateReady, events, callbacks)
	// create task
	task := Task{
		name:     name,
		task:     iface,
		fsm:      FSM,
		paused:   new(int32),
		pausedCh: make(chan struct{}, 1),
	}
	task.ctx, task.cancel = context.WithCancel(context.Background())
	return &task
}

// Start is used to start current task.
func (task *Task) Start() (err error) {
	task.startOnce.Do(func() {
		defer func() {
			if err != nil {
				task.Cancel()
			} else {
				task.clean()
			}
		}()
		defer func() {
			if r := recover(); r != nil {
				buf := xpanic.Log(r, "Task.Start")
				err = errors.New(buf.String())
			}
		}()
		if !task.checkStart() {
			err = errors.New("task canceled")
			return
		}
		err = task.prepare()
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

func (task *Task) checkStart() bool {
	task.mu.Lock()
	defer task.mu.Unlock()
	if task.finished {
		return false
	}
	err := task.fsm.Event(EventStart)
	if err != nil {
		panic(fmt.Sprintf(panicFormat, err))
	}
	return true
}

func (task *Task) prepare() error {
	err := task.task.Prepare(task.ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to prepare task")
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
		panic(fmt.Sprintf(panicFormat, err))
	}
	return true
}

func (task *Task) process() error {
	err := task.task.Process(task.ctx, task)
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
		panic(fmt.Sprintf(panicFormat, err))
	}
	return nil
}

// clean will be call once after task finish(include Cancel before Start)
func (task *Task) clean() {
	task.cleanOnce.Do(func() {
		task.task.Clean()
	})
}

// Pause is used to pause process progress.
func (task *Task) Pause() error {
	task.mu.Lock()
	defer task.mu.Unlock()
	if task.finished {
		return nil
	}
	if !atomic.CompareAndSwapInt32(task.paused, 0, 1) {
		return nil
	}
	err := task.fsm.Event(EventPause)
	if err != nil {
		atomic.StoreInt32(task.paused, 0)
		return err
	}
	return nil
}

// Continue is used to continue current task.
func (task *Task) Continue() error {
	task.mu.Lock()
	defer task.mu.Unlock()
	if task.finished {
		return nil
	}
	if !atomic.CompareAndSwapInt32(task.paused, 1, 0) {
		return nil
	}
	select {
	case task.pausedCh <- struct{}{}:
		err := task.fsm.Event(EventContinue)
		if err != nil {
			atomic.StoreInt32(task.paused, 1)
			return err
		}
	default:
	}
	return nil
}

// Cancel is used to cancel current task.
func (task *Task) Cancel() {
	task.cancelOnce.Do(func() {
		defer task.clean()
		defer func() {
			if r := recover(); r != nil {
				xpanic.Log(r, "Task.Cancel")
			}
		}()

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
			panic(fmt.Sprintf(panicFormat, err))
		}
	})
}

// Paused is used to check current task is paused in process function.
func (task *Task) Paused() {
	if atomic.LoadInt32(task.paused) != 1 {
		return
	}
	select {
	case <-task.pausedCh:
	case <-task.ctx.Done():
	}
}

// Canceled is used to check current task is canceled, it is a shortcut.
func (task *Task) Canceled() bool {
	select {
	case <-task.ctx.Done():
		return true
	default:
		return false
	}
}

// Name is used to get the name of current task.
func (task *Task) Name() string {
	return task.name
}

// State is used to get the state about current task.
func (task *Task) State() string {
	return task.fsm.Current()
}

// Progress is used to get the progress about current task.
func (task *Task) Progress() float32 {
	return task.task.Progress()
}

// Detail is used to get the detail about current task.
func (task *Task) Detail() string {
	return task.task.Detail()
}