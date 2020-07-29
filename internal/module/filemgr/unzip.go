package filemgr

import (
	"archive/zip"
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/looplab/fsm"
	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/module/task"
	"project/internal/xpanic"
)

// unZipTask implement task.Interface that is used to extract files from one zip file.
// It can pause in progress and get current progress and detail information.
type unZipTask struct {
	errCtrl  ErrCtrl
	src      string // zip file absolute  path
	dst      string // destination path to store extract files
	paths    []string
	pathsLen int

	skipDirs []string // store skipped directories

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
func NewUnZipTask(errCtrl ErrCtrl, callbacks fsm.Callbacks, src, dst string, path ...string) *task.Task {
	ut := unZipTask{
		errCtrl:    errCtrl,
		src:        src,
		dst:        dst,
		paths:      path,
		pathsLen:   len(path),
		current:    big.NewFloat(0),
		total:      big.NewFloat(0),
		stopSignal: make(chan struct{}),
	}
	return task.New(TaskNameUnZip, &ut, callbacks)
}

// Prepare is used to check destination is not exist or a file.
func (ut *unZipTask) Prepare(context.Context) error {
	stats, err := checkSrcDstPath(ut.src, ut.dst)
	if err != nil {
		return err
	}
	if stats.SrcStat.IsDir() {
		return errors.Errorf("source path %s is a directory", stats.SrcAbs)
	}
	if stats.DstStat != nil && !stats.DstStat.IsDir() {
		return errors.Errorf("destination path %s is a file", stats.DstAbs)
	}
	ut.src = stats.SrcAbs
	ut.dst = stats.DstAbs
	go ut.watcher()
	return nil
}

func (ut *unZipTask) Process(ctx context.Context, task *task.Task) error {
	defer ut.updateDetail("finished")
	// open and read zip file
	ut.updateDetail("open zip file")
	zipFile, err := zip.OpenReader(ut.src)
	if err != nil {
		return err
	}
	defer func() { _ = zipFile.Close() }()
	if ut.pathsLen == 0 {
		return ut.extractAll(ctx, task)
	}
	return ut.extractPart(ctx, task)
}

func (ut *unZipTask) extractAll(ctx context.Context, task *task.Task) error {

	return nil
}

func (ut *unZipTask) extractPart(ctx context.Context, task *task.Task) error {
	return nil
}

// Progress is used to get progress about current unzip task.
//
// collect: "0%"
// unzip:   "15.22%|current/total|128 MB/s"
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

// Detail is used to get detail about unzip task.
// open zip file:
//   open zip file
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

func (ut *unZipTask) updateDetail(detail string) {
	ut.rwm.Lock()
	defer ut.rwm.Unlock()
	ut.detail = detail
}

// watcher is used to calculate current copy speed.
func (ut *unZipTask) watcher() {
	defer func() {
		if r := recover(); r != nil {
			xpanic.Log(r, "unZipTask.watcher")
		}
	}()
	ticker := time.NewTicker(time.Second / time.Duration(len(ut.speeds)))
	defer ticker.Stop()
	current := new(big.Float)
	index := -1
	for {
		select {
		case <-ticker.C:
			index++
			if index >= len(ut.speeds) {
				index = 0
			}
			ut.watchSpeed(current, index)
		case <-ut.stopSignal:
			return
		}
	}
}

func (ut *unZipTask) watchSpeed(current *big.Float, index int) {
	ut.rwm.Lock()
	defer ut.rwm.Unlock()
	delta := new(big.Float).Sub(ut.current, current)
	current.Add(current, delta)
	// update speed
	ut.speeds[index], _ = delta.Uint64()
	if ut.full {
		ut.speed = 0
		for i := 0; i < len(ut.speeds); i++ {
			ut.speed += ut.speeds[i]
		}
		return
	}
	if index == len(ut.speeds)-1 {
		ut.full = true
	}
	// calculate average speed
	var speed float64 // current speed
	for i := 0; i < index+1; i++ {
		speed += float64(ut.speeds[i])
	}
	ut.speed = uint64(speed / float64(index+1) * float64(len(ut.speeds)))
}

// Clean is used to send stop signal to watcher.
func (ut *unZipTask) Clean() {
	close(ut.stopSignal)
}

// UnZip is used to create a unzip task to extract files from zip file.
func UnZip(errCtrl ErrCtrl, src, dst string, paths ...string) error {
	return UnZipWithContext(context.Background(), errCtrl, src, dst, paths...)
}

// UnZipWithContext is used to create a unzip task with context to extract files from zip file.
func UnZipWithContext(ctx context.Context, errCtrl ErrCtrl, src, dst string, paths ...string) error {
	ut := NewUnZipTask(errCtrl, nil, src, dst, paths...)
	return startTask(ctx, ut, "UnZip")
}
