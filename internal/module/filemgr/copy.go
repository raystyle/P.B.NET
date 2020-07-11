package filemgr

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/looplab/fsm"

	"project/internal/convert"
	"project/internal/module/task"
)

// copyTask implement task.Interface that is used to copy src to dst directory.
// It can pause in progress and get current progress and detail information.
type copyTask struct {
	errCtrl ErrCtrl
	src     string
	dst     string
	stats   *srcDstStat

	current *big.Float
	total   *big.Float
	detail  string
	rwm     sync.RWMutex
}

// NewCopyTask is used to create a copy task that implement task.Interface.
func NewCopyTask(errCtrl ErrCtrl, src, dst string, callbacks fsm.Callbacks) *task.Task {
	ct := copyTask{
		errCtrl: errCtrl,
		src:     src,
		dst:     dst,
		current: big.NewFloat(0),
		total:   big.NewFloat(0),
	}
	return task.New(TaskNameCopy, &ct, callbacks)
}

// Prepare will check src and dst path.
func (ct *copyTask) Prepare(context.Context) error {
	stats, err := checkSrcDstPath(ct.src, ct.dst)
	if err != nil {
		return err
	}
	ct.stats = stats
	return nil
}

func (ct *copyTask) Process(ctx context.Context, task *task.Task) error {
	if ct.stats.srcIsFile {
		return ct.copySrcFile(ctx, task)
	}
	return nil
}

func (ct *copyTask) copySrcFile(ctx context.Context, task *task.Task) error {
	_, srcFileName := filepath.Split(ct.stats.srcAbs)
	var (
		dstFileName string
		dstStat     os.FileInfo
	)
	if ct.stats.dstStat != nil { // dst is exists
		// copyFile will handle the same file, dir
		//
		// copy "a.exe" -> "C:\ExistDir"
		// "a.exe" -> "C:\ExistDir\a.exe"
		if ct.stats.dstStat.IsDir() {
			dstFileName = filepath.Join(ct.stats.dstAbs, srcFileName)
			stat, err := stat(dstFileName)
			if err != nil {
				return err
			}
			dstStat = stat
		} else {
			dstFileName = ct.stats.dstAbs
			dstStat = ct.stats.dstStat
		}
	} else { // dst is doesn't exists
		last := ct.dst[len(ct.dst)-1]
		if os.IsPathSeparator(last) { // is a directory path
			err := os.MkdirAll(ct.stats.dstAbs, 0750)
			if err != nil {
				return err
			}
			dstFileName = filepath.Join(ct.stats.dstAbs, srcFileName)
		} else { // is a file path
			dir, _ := filepath.Split(ct.stats.dstAbs)
			err := os.MkdirAll(dir, 0750)
			if err != nil {
				return err
			}
			dstFileName = ct.stats.dstAbs
		}
	}
	stats := &srcDstStat{
		srcAbs:  ct.stats.srcAbs,
		dstAbs:  dstFileName,
		srcStat: ct.stats.srcStat,
		dstStat: dstStat,
	}
	// update progress
	ct.updateTotal(ct.stats.srcStat.Size(), true)
	return ct.copyFile(ctx, task, stats)
}

// collect will collect directory information.
func (ct *copyTask) collect() error {
	return nil
}

func (ct *copyTask) copyFile(ctx context.Context, task *task.Task, stats *srcDstStat) (err error) {
	// update detail information
	//
	// name: test.dat size: 1.127MB
	// src:  C:\testdata\test.dat
	// dst:  D:\test\test.dat
	const detailFormat = "name: %s size: %s\nsrc:  %s\ndst:  %s"
	_, srcFileName := filepath.Split(stats.srcAbs)
	srcSize := convert.ByteToString(uint64(stats.srcStat.Size()))
	ct.updateDetail(fmt.Sprintf(detailFormat, srcFileName, srcSize, stats.srcAbs, stats.dstAbs))
	// check dst file is exist
	if stats.dstStat != nil {
		if stats.dstStat.IsDir() {
			retry, err := notice(task, func() (bool, error) {
				return noticeSameFileDir(ctx, ct.errCtrl, stats)
			})
			if retry {
				return ct.retryCopyFile(ctx, task, stats)
			}
			// update total
			ct.updateCurrent(stats.srcStat.Size(), true)
			return err
		}
		replace, err := notice(task, func() (bool, error) {
			return noticeSameFile(ctx, ct.errCtrl, stats)
		})
		if !replace {
			ct.updateCurrent(stats.srcStat.Size(), true)
			return err
		}
	}
	// check copy file error, and maybe retry copy file.
	var copied int64
	defer func() {
		if err != nil && err != context.Canceled {
			// reset current
			ct.updateCurrent(copied, false)
			var retry bool
			retry, err = notice(task, func() (bool, error) {
				return noticeFailedToCopy(ctx, ct.errCtrl, stats, err)
			})
			if retry {
				err = ct.retryCopyFile(ctx, task, stats)
			} else if err == nil {
				ct.updateCurrent(stats.srcStat.Size(), true)
			}
		}
	}()
	// open src file
	srcFile, err := os.Open(stats.srcAbs)
	if err != nil {
		return
	}
	defer func() { _ = srcFile.Close() }()
	// open dst file
	perm := stats.srcStat.Mode().Perm()
	dstFile, err := os.OpenFile(stats.dstAbs, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return
	}
	defer func() { _ = dstFile.Close() }()
	// copy file
	copied, err = ioCopy(task, ct.ioCopyAdd, dstFile, srcFile)
	if err != nil {
		_ = dstFile.Close()
		_ = os.Remove(stats.dstAbs)
		return
	}
	// sync
	err = dstFile.Sync()
	if err != nil {
		return
	}
	// set the modification time about the dst file
	modTime := stats.srcStat.ModTime()
	return os.Chtimes(stats.dstAbs, modTime, modTime)
}

func (ct *copyTask) ioCopyAdd(delta int64) {
	ct.updateCurrent(delta, true)
}

// retryCopyFile will update src and dst file stat.
func (ct *copyTask) retryCopyFile(ctx context.Context, task *task.Task, stats *srcDstStat) error {
	var err error
	stats.srcStat, err = os.Stat(stats.srcAbs)
	if err != nil {
		return err
	}
	stats.dstStat, err = stat(stats.dstAbs)
	if err != nil {
		return err
	}
	return ct.copyFile(ctx, task, stats)
}

func (ct *copyTask) updateCurrent(delta int64, add bool) {
	ct.rwm.Lock()
	defer ct.rwm.Unlock()
	d := new(big.Float).SetInt64(delta)
	if add {
		ct.current.Add(ct.current, d)
	} else {
		ct.current.Sub(ct.current, d)
	}
}

func (ct *copyTask) updateTotal(delta int64, add bool) {
	ct.rwm.Lock()
	defer ct.rwm.Unlock()
	d := new(big.Float).SetInt64(delta)
	if add {
		ct.total.Add(ct.current, d)
	} else {
		ct.total.Sub(ct.current, d)
	}
}

// Progress will be 0%(in collect), 100%(finish) or 15.22%|[current]/[total]
func (ct *copyTask) Progress() string {
	ct.rwm.RLock()
	defer ct.rwm.RUnlock()
	// prevent / 0
	if ct.total.Cmp(zeroFloat) == 0 {
		return "0%"
	}
	switch ct.current.Cmp(ct.total) {
	case 0:
		return "100%"
	case 1: // current > total
		current := ct.current.Text('G', 64)
		total := ct.total.Text('G', 64)
		return fmt.Sprintf("err: current[%s] > total[%s]", current, total)
	}
	value := new(big.Float).Quo(ct.current, ct.total)
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
	str := strconv.FormatFloat(result*100, 'f', -1, 64) + "%"
	// add |[current]/[total]
	current := ct.current.Text('G', 64)
	total := ct.total.Text('G', 64)
	return str + fmt.Sprintf("|[%s]/[%s]", current, total)
}

func (ct *copyTask) updateDetail(detail string) {
	ct.rwm.Lock()
	defer ct.rwm.Unlock()
	ct.detail = detail
}

func (ct *copyTask) Detail() string {
	ct.rwm.RLock()
	defer ct.rwm.RUnlock()
	return ct.detail
}

func (ct *copyTask) Clean() {}

// Copy is used to create a copyTask to copy src to dst.
func Copy(errCtrl ErrCtrl, src, dst string) error {
	return NewCopyTask(errCtrl, src, dst, nil).Start()
}
