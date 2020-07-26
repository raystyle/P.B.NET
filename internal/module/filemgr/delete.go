package filemgr

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"

	"github.com/looplab/fsm"

	"project/internal/convert"
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

// NewDeleteTask is used to create a delete task that implement task.Interface.
func NewDeleteTask(errCtrl ErrCtrl, callbacks fsm.Callbacks, src ...string) *task.Task {
	dt := deleteTask{
		errCtrl:    errCtrl,
		src:        src,
		current:    big.NewFloat(0),
		total:      big.NewFloat(0),
		stopSignal: make(chan struct{}),
	}
	return task.New(TaskNameDelete, &dt, callbacks)
}

// Prepare is used to check directory about source paths is equal.
func (dt *deleteTask) Prepare(context.Context) error {
	return nil
}

func (dt *deleteTask) Process(ctx context.Context, task *task.Task) error {
	return nil
}

func (dt *deleteTask) updateCurrent(add bool) {
	dt.rwm.Lock()
	defer dt.rwm.Unlock()
	if add {
		dt.current.Add(dt.current, deleteDelta)
	} else {
		dt.current.Sub(dt.current, deleteDelta)
	}
}

func (dt *deleteTask) updateTotal(add bool) {
	dt.rwm.Lock()
	defer dt.rwm.Unlock()
	if add {
		dt.total.Add(dt.total, deleteDelta)
	} else {
		dt.total.Sub(dt.total, deleteDelta)
	}
}

// Progress is used to get progress about current delete task.
//
// collect: "0%"
// delete:  "15.22%|current/total|4,523 file/s"
// finish:  "100%"
func (dt *deleteTask) Progress() string {
	dt.rwm.RLock()
	defer dt.rwm.RUnlock()
	// prevent / 0
	if dt.total.Cmp(zeroFloat) == 0 {
		return "0%"
	}
	switch dt.current.Cmp(dt.total) {
	case 0: // current == total
		return "100%"
	case 1: // current > total
		current := dt.current.Text('G', 64)
		total := dt.total.Text('G', 64)
		return fmt.Sprintf("err: current %s > total %s", current, total)
	}
	value := new(big.Float).Quo(dt.current, dt.total)
	// split result
	text := value.Text('G', 64)
	if len(text) > 6 { // 0.999999999...999 -> 0.9999
		text = text[:6]
	}
	// format result
	result, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return fmt.Sprintf("err: %s", err)
	}
	// 0.9999 -> 99.99%
	progress := strconv.FormatFloat(result*100, 'f', -1, 64)
	offset := strings.Index(progress, ".")
	if offset != -1 {
		if len(progress[offset+1:]) > 2 {
			progress = progress[:offset+3]
		}
	}
	// progress|current/total|speed
	current := dt.current.Text('G', 64)
	total := dt.total.Text('G', 64)
	speed := convert.FormatNumber(strconv.FormatUint(dt.speed, 10))
	return fmt.Sprintf("%s%%|%s/%s|%s file/s", progress, current, total, speed)
}

func (dt *deleteTask) updateDetail(detail string) {
	dt.rwm.Lock()
	defer dt.rwm.Unlock()
	dt.detail = detail
}

// Detail is used to get detail about delete task.
//
// collect dir info:
//   collect directory information
//   path: C:\testdata\test
//
// delete file:
//   delete file, name: test.dat
//   path: C:\testdata\test.dat
func (dt *deleteTask) Detail() string {
	dt.rwm.RLock()
	defer dt.rwm.RUnlock()
	return dt.detail
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
