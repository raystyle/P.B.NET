package filemgr

import (
	"context"
	"math/big"
	"sync"

	"github.com/looplab/fsm"

	"project/internal/module/task"
)

// unZipTask implement task.Interface that is used to extract files from one zip file.
// It can pause in progress and get current progress and detail information.
type unZipTask struct {
	errCtrl  ErrCtrl
	src      string // zip file path
	dst      string // destination path to store extract files
	files    []string
	filesLen int

	// about progress, detail and speed
	current *big.Float
	total   *big.Float
	detail  string
	speed   uint64
	speeds  [10]uint64
	full    bool
	rwm     sync.RWMutex

	// control speed watcher
	stopSignal chan struct{}
}

// NewUnZipTask is used to create a unzip task that implement task.Interface.
// If files is nil, extract all files from source zip file.
func NewUnZipTask(errCtrl ErrCtrl, callbacks fsm.Callbacks, src, dst string, files ...string) *task.Task {
	ut := unZipTask{
		errCtrl:    errCtrl,
		src:        src,
		dst:        dst,
		files:      files,
		filesLen:   len(files),
		current:    big.NewFloat(0),
		total:      big.NewFloat(0),
		stopSignal: make(chan struct{}),
	}
	return task.New(TaskNameUnZip, &ut, callbacks)
}

func (ut *unZipTask) Prepare(context.Context) error {
	return nil
}

func (ut *unZipTask) Process(ctx context.Context, task *task.Task) error {
	return nil
}

func (ut *unZipTask) Progress() string {
	return ""
}

func (ut *unZipTask) Detail() string {
	ut.rwm.RLock()
	defer ut.rwm.RUnlock()
	return ut.detail
}

// Clean is used to send stop signal to watcher.
func (ut *unZipTask) Clean() {
	close(ut.stopSignal)
}
