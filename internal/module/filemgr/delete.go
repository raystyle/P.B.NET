package filemgr

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/looplab/fsm"
	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/module/task"
	"project/internal/xpanic"
)

// deleteTask is implement task.Interface that is used to delete file or files in one
// directory. It can pause in progress and get current progress and detail information.
type deleteTask struct {
	errCtrl ErrCtrl
	src     []string
	srcLen  int

	roots    []*file          // store all directories and files will delete
	dirs     map[string]*file // for search dir faster, key is path
	skipDirs []string         // store skipped directories

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

// NewDeleteTask is used to create a delete task that implement task.Interface.
func NewDeleteTask(errCtrl ErrCtrl, callbacks fsm.Callbacks, src ...string) *task.Task {
	dt := deleteTask{
		errCtrl:    errCtrl,
		src:        src,
		srcLen:     len(src),
		current:    big.NewFloat(0),
		total:      big.NewFloat(0),
		stopSignal: make(chan struct{}),
	}
	return task.New(TaskNameDelete, &dt, callbacks)
}

// Prepare is used to check directory about source paths is same.
func (dt *deleteTask) Prepare(context.Context) error {
	if dt.srcLen == 0 {
		return errors.New("empty path")
	}
	// check path is exists
	paths := make(map[string]struct{}, dt.srcLen)
	var dir string
	for i := 0; i < dt.srcLen; i++ {
		// make sure all source path is absolute
		srcAbs, err := filepath.Abs(dt.src[i])
		if err != nil {
			return errors.Wrap(err, "failed to get absolute file path")
		}
		dt.src[i] = srcAbs
		if i == 0 {
			paths[srcAbs] = struct{}{}
			dir = filepath.Dir(srcAbs)
			continue
		}
		_, ok := paths[srcAbs]
		if ok {
			const format = "appear the same path \"%s\""
			return errors.Errorf(format, srcAbs)
		}
		d := filepath.Dir(srcAbs)
		if d != dir {
			const format = "split directory about source \"%s\" is different with \"%s\""
			return errors.Errorf(format, srcAbs, dt.src[0])
		}
		paths[srcAbs] = struct{}{}
	}
	dt.roots = make([]*file, dt.srcLen)
	dt.dirs = make(map[string]*file, dt.srcLen/4)
	go dt.watcher()
	return nil
}

func (dt *deleteTask) Process(ctx context.Context, task *task.Task) error {
	defer dt.updateDetail("finished")
	for i := 0; i < dt.srcLen; i++ {
		err := dt.collectDirInfo(ctx, task, i)
		if err != nil {
			return err
		}
	}
	for i := 0; i < dt.srcLen; i++ {
		err := dt.deleteRoot(ctx, task, dt.roots[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (dt *deleteTask) collectDirInfo(ctx context.Context, task *task.Task, i int) error {
	src := dt.src[i]
	var (
		cDir  string // current directory
		cFile *file  // current file
	)
	walkFunc := func(srcAbs string, srcStat os.FileInfo, err error) error {
		if err != nil {
			const format = "failed to walk \"%s\" in \"%s\": %s"
			err = fmt.Errorf(format, srcAbs, src, err)
			skip, ne := noticeFailedToCollect(ctx, task, dt.errCtrl, srcAbs, err)
			if skip {
				return filepath.SkipDir
			}
			return ne
		}
		// check task is canceled
		if task.Canceled() {
			return context.Canceled
		}
		f := &file{
			path: srcAbs,
			stat: srcStat,
		}
		isDir := srcStat.IsDir()
		// check is root directory
		if dt.roots[i] == nil {
			dt.roots[i] = f
			// check root is file
			if !isDir {
				dt.updateTotal()
				return nil
			}
			dt.dirs[srcAbs] = f
			// set current data
			cDir = srcAbs
			cFile = f
			return nil
		}
		// update detail and total
		dir := filepath.Dir(srcAbs)
		if dir != cDir {
			cDir = dir
			cFile = dt.dirs[dir]
		}
		cFile.files = append(cFile.files, f)
		if isDir {
			cDir = srcAbs
			cFile = f
			dt.dirs[srcAbs] = f
			// collecting directory information
			// path: C:\testdata\test
			const format = "collect directory information\npath: %s"
			dt.updateDetail(fmt.Sprintf(format, srcAbs))
			return nil
		}
		// collecting file information
		// path: C:\testdata\test
		const format = "collect file information\npath: %s"
		dt.updateDetail(fmt.Sprintf(format, srcAbs))
		dt.updateTotal()
		return nil
	}
	return filepath.Walk(src, walkFunc)
}

func (dt *deleteTask) deleteRoot(ctx context.Context, task *task.Task, root *file) error {
	// skip root directory
	// set fake progress for pass progress check
	if root == nil {
		dt.rwm.Lock()
		defer dt.rwm.Unlock()
		dt.current.Add(dt.current, deleteDelta)
		dt.total.Add(dt.total, deleteDelta)
		return nil
	}
	// check root is directory
	if !root.stat.IsDir() {
		_, err := dt.deleteDirFile(ctx, task, root)
		if err != nil {
			return errors.WithMessage(err, "failed to delete file")
		}
		return nil
	}
	// delete all directories and files in root directory
	_, err := dt.deleteDir(ctx, task, root)
	if err != nil {
		return errors.WithMessage(err, "failed to delete directory")
	}
	return nil
}

// returned bool is skipped this file.
func (dt *deleteTask) deleteDir(ctx context.Context, task *task.Task, dir *file) (bool, error) {
	var (
		skipped bool
		err     error
	)
	for _, file := range dir.files {
		// check task is canceled
		if task.Canceled() {
			return false, context.Canceled
		}
		var skip bool
		if file.stat.IsDir() {
			skip, err = dt.deleteDir(ctx, task, file)
		} else {
			skip, err = dt.deleteDirFile(ctx, task, file)
		}
		if err != nil {
			return false, err
		}
		if skip && !skipped {
			skipped = true
		}
	}
	// if delete all files successfully, delete the directory
	if !skipped {
		return false, os.Remove(dir.path)
	}
	return true, nil
}

// returned bool is skipped this file.
func (dt *deleteTask) deleteDirFile(ctx context.Context, task *task.Task, file *file) (bool, error) {
	// skip file if it in skipped directories
	for i := 0; i < len(dt.skipDirs); i++ {
		if strings.HasPrefix(file.path, dt.skipDirs[i]) {
			dt.updateCurrent()
			return true, nil
		}
	}
	// update current task detail, output:
	//   delete file, name: test.dat
	//   path: C:\testdata\test.dat
	//   modify time: 2020-07-27 19:12:17
	const format = "delete file, name: %s\npath: %s\nmodify time: %s"
	fileName := filepath.Base(file.path)
	modTime := file.stat.ModTime().Format(logger.TimeLayout)
	dt.updateDetail(fmt.Sprintf(format, fileName, file.path, modTime))
retry:
	// check task is canceled
	if task.Canceled() {
		return false, context.Canceled
	}
	// delete file
	err := os.Remove(file.path)
	if err != nil {
		retry, ne := noticeFailedToDelete(ctx, task, dt.errCtrl, file.path, err)
		if retry {
			goto retry
		}
		if ne != nil {
			return false, ne
		}
		dt.updateCurrent()
		return true, nil
	}
	dt.updateCurrent()
	return false, nil
}

func (dt *deleteTask) updateCurrent() {
	dt.rwm.Lock()
	defer dt.rwm.Unlock()
	dt.current.Add(dt.current, deleteDelta)
}

func (dt *deleteTask) updateTotal() {
	dt.rwm.Lock()
	defer dt.rwm.Unlock()
	dt.total.Add(dt.total, deleteDelta)
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
//   modify time: 2020-07-27 19:12:17
func (dt *deleteTask) Detail() string {
	dt.rwm.RLock()
	defer dt.rwm.RUnlock()
	return dt.detail
}

// watcher is used to calculate current delete speed.
func (dt *deleteTask) watcher() {
	defer func() {
		if r := recover(); r != nil {
			xpanic.Log(r, "deleteTask.watcher")
		}
	}()
	ticker := time.NewTicker(time.Second / time.Duration(len(dt.speeds)))
	defer ticker.Stop()
	current := new(big.Float)
	index := -1
	for {
		select {
		case <-ticker.C:
			index++
			if index >= len(dt.speeds) {
				index = 0
			}
			dt.watchSpeed(current, index)
		case <-dt.stopSignal:
			return
		}
	}
}

func (dt *deleteTask) watchSpeed(current *big.Float, index int) {
	dt.rwm.Lock()
	defer dt.rwm.Unlock()
	delta := new(big.Float).Sub(dt.current, current)
	current.Add(current, delta)
	// update speed
	dt.speeds[index], _ = delta.Uint64()
	if dt.full {
		dt.speed = 0
		for i := 0; i < len(dt.speeds); i++ {
			dt.speed += dt.speeds[i]
		}
		return
	}
	if index == len(dt.speeds)-1 {
		dt.full = true
	}
	// calculate average speed
	var speed uint64 // current speed
	for i := 0; i < index+1; i++ {
		speed += dt.speeds[i]
	}
	dt.speed = speed / uint64(index+1) * uint64(len(dt.speeds))
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
