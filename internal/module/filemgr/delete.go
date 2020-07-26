package filemgr

import (
	"context"
	"math/big"
	"sync"

	"github.com/looplab/fsm"

	"project/internal/module/task"
)

// deleteTask is implement task.Interface that is used to delete file or files in one
// directory. It can pause in progress and get current progress and detail information.
type deleteTask struct {
	errCtrl ErrCtrl
	src     []string

	// store all files will delete
	files    []*fileStat
	skipDirs []string

	current *big.Int
	total   *big.Int
	detail  string
	speed   uint64
	speeds  [10]uint64
	full    bool
	rwm     sync.RWMutex

	// control speed watcher
	stopSignal chan struct{}
}

// NewDeleteTask is used to create a delete task that implement task.Interface.
func NewDeleteTask(errCtrl ErrCtrl, callbacks fsm.Callbacks, src ...string) *task.Task {
	dt := deleteTask{
		errCtrl:    errCtrl,
		src:        src,
		current:    big.NewInt(0),
		total:      big.NewInt(0),
		stopSignal: make(chan struct{}),
	}
	return task.New(TaskNameDelete, &dt, callbacks)
}

func (dt *deleteTask) Prepare(context.Context) error {
	return nil
}

func (dt *deleteTask) Process(ctx context.Context, task *task.Task) error {
	return nil
}

func (dt *deleteTask) Progress() string {
	return ""
}

func (dt *deleteTask) Detail() string {
	return ""
}

// Clean is used to send stop signal to watcher.
func (dt *deleteTask) Clean() {
	close(dt.stopSignal)
}

// Delete is used to create a delete task to delete paths.
func Delete(errCtrl ErrCtrl, src ...string) error {
	return DeleteWithContext(context.Background(), errCtrl, src...)
}

// DeleteWithContext is used to create a delete task with context to delete paths.
func DeleteWithContext(ctx context.Context, errCtrl ErrCtrl, src ...string) error {
	dt := NewDeleteTask(errCtrl, nil, src...)
	return startTask(ctx, dt, "Delete")
}
