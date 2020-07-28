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

func (ut *unZipTask) updateCurrent(delta int64, add bool) {
	ut.rwm.Lock()
	defer ut.rwm.Unlock()
	d := new(big.Float).SetInt64(delta)
	if add {
		ut.current.Add(ut.current, d)
	} else {
		ut.current.Sub(ut.current, d)
	}
}

func (ut *unZipTask) updateTotal(delta int64, add bool) {
	ut.rwm.Lock()
	defer ut.rwm.Unlock()
	d := new(big.Float).SetInt64(delta)
	if add {
		ut.total.Add(ut.total, d)
	} else {
		ut.total.Sub(ut.total, d)
	}
}

// Progress is used to get progress about current unzip task.
//
// collect: "0%"
// copy:    "15.22%|current/total|128 MB/s"
// finish:  "100%"
func (ut *unZipTask) Progress() string {
	ut.rwm.RLock()
	defer ut.rwm.RUnlock()
	// prevent / 0
	if ut.total.Cmp(zeroFloat) == 0 {
		return "0%"
	}
	switch ut.current.Cmp(ut.total) {
	case 0: // current == total
		return "100%"
	case 1: // current > total
		current := ut.current.Text('G', 64)
		total := ut.total.Text('G', 64)
		return fmt.Sprintf("err: current %s > total %s", current, total)
	}
	value := new(big.Float).Quo(ut.current, ut.total)
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
	current := ut.current.Text('G', 64)
	total := ut.total.Text('G', 64)
	speed := convert.FormatByte(ut.speed)
	return fmt.Sprintf("%s%%|%s/%s|%s/s", progress, current, total, speed)
}

func (ut *unZipTask) updateDetail(detail string) {
	ut.rwm.Lock()
	defer ut.rwm.Unlock()
	ut.detail = detail
}

// Detail is used to get detail about unzip task.
//
// collect file info:
//   collect file information
//   path: testdata/test.dat
//
// extract file:
//   extract file, name: test.dat
//   src: testdata/test.dat
//   dst: C:\testdata\test.dat
func (ut *unZipTask) Detail() string {
	ut.rwm.RLock()
	defer ut.rwm.RUnlock()
	return ut.detail
}

// Clean is used to send stop signal to watcher.
func (ut *unZipTask) Clean() {
	close(ut.stopSignal)
}

// Delete is used to create a delete task to delete paths.
func UnZip(errCtrl ErrCtrl, src, dst string, files ...string) error {
	return UnZipWithContext(context.Background(), errCtrl, src, dst, files...)
}

// DeleteWithContext is used to create a delete task with context to delete paths.
func UnZipWithContext(ctx context.Context, errCtrl ErrCtrl, src, dst string, files ...string) error {
	dt := NewUnZipTask(errCtrl, nil, src, dst, files...)
	return startTask(ctx, dt, "UnZip")
}
