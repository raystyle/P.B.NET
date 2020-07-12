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

	"github.com/looplab/fsm"
	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/module/task"
	"project/internal/xpanic"
)

// copyTask implement task.Interface that is used to copy src to dst directory.
// It can pause in progress and get current progress and detail information.
type copyTask struct {
	errCtrl ErrCtrl
	src     string
	dst     string
	stats   *srcDstStat

	files    []*fileStat
	skipDirs []string

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
	return ct.copySrcDir(ctx, task)
}

// copySrcFile is used to copy single file to a path.
//
// new path is a dir  and exist
// new path is a file and exist
// new path is a dir  and not exist
// new path is a file and not exist
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

// copy C:\test -> D:\test2
// -- copy C:\test\file.dat -> C:\test2\file.dat
func (ct *copyTask) copySrcDir(ctx context.Context, task *task.Task) error {
	err := ct.collectDirInfo(ctx, task)
	if err != nil {
		return err
	}
	return ct.copyDirFiles(ctx, task)
}

// collect will collect directory information for calculate total size.
func (ct *copyTask) collectDirInfo(ctx context.Context, task *task.Task) error {
	ct.files = make([]*fileStat, 64)
	walkFunc := func(srcAbs string, srcStat os.FileInfo, err error) error {
		// for retry
		var walkFailed bool
	retry:
		// check task is canceled
		if task.Canceled() {
			return context.Canceled
		}
		if walkFailed {
			srcStat, err = os.Stat(srcAbs)
		}
		if err != nil {
			const format = "failed to walk \"%s\" in \"%s\": %s"
			ne := fmt.Errorf(format, srcAbs, ct.stats.srcAbs, err)
			retry, err := noticeFailedToCollect(ctx, task, ct.errCtrl, srcAbs, ne)
			if retry {
				walkFailed = true
				goto retry
			}
			if err != nil {
				return err
			}
			return filepath.SkipDir
		}
		ct.files = append(ct.files, &fileStat{
			path: srcAbs,
			stat: srcStat,
		})
		// update detail and total size
		if srcStat.IsDir() {
			// collecting directory information
			// path: C:\testdata\test
			const format = "collecting directory information\npath: %s"
			ct.updateDetail(fmt.Sprintf(format, srcAbs))
		} else {
			// collecting directory information
			// path: C:\testdata\test
			const format = "collecting file information\npath: %s"
			ct.updateDetail(fmt.Sprintf(format, srcAbs))
			ct.updateTotal(srcStat.Size(), true)
		}
		return nil
	}
	return filepath.Walk(ct.stats.srcAbs, walkFunc)
}

func (ct *copyTask) copyDirFiles(ctx context.Context, task *task.Task) error {
	// check root path, and make directory if target path is not exists
	// C:\test -> D:\test[exist]
	if ct.stats.dstStat == nil {
		err := os.MkdirAll(ct.stats.dstAbs, ct.stats.srcStat.Mode().Perm())
		if err != nil {
			return errors.Wrap(err, "failed to create destination directory")
		}
	}
	// must check root path special, otherwise will appear zero relative path
	for _, file := range ct.files[1:] {
		err := ct.copyDirFile(ctx, task, file)
		if err != nil {
			return errors.Wrap(err, "failed to copy file")
		}
	}
	return nil
}

func (ct *copyTask) copyDirFile(ctx context.Context, task *task.Task, file *fileStat) error {
	// skip file in skipped directories
	for i := 0; i < len(ct.skipDirs); i++ {
		if strings.HasPrefix(file.path, ct.skipDirs[i]) {
			ct.updateCurrent(file.stat.Size(), true)
			return nil
		}
	}
	// C:\test\a.exe -> a.exe
	// C:\test\dir\a.exe -> dir\a.exe
	relativePath := strings.ReplaceAll(file.path, ct.stats.srcAbs, "")
	relativePath = string([]rune(relativePath)[1:]) // remove the first "\" or "/"
	dstAbs := filepath.Join(ct.stats.dstAbs, relativePath)
retry:
	// check task is canceled
	if task.Canceled() {
		return context.Canceled
	}
	// dstStat maybe updated
	dstStat, err := stat(dstAbs)
	if err != nil {
		retry, err := noticeFailedToCopyDir(ctx, task, ct.errCtrl, dstAbs, err)
		if retry {
			goto retry
		}
		if err != nil {
			return err
		}
		ct.updateCurrent(file.stat.Size(), true)
		return nil
	}
	newStats := &srcDstStat{
		srcAbs:  file.path,
		dstAbs:  dstAbs,
		srcStat: file.stat,
		dstStat: dstStat,
	}
	if file.stat.IsDir() {
		if dstStat == nil {
			return ct.mkDir(ctx, task, file, dstAbs)
		}
		if !dstStat.IsDir() {
			retry, err := noticeSameDirFile(ctx, task, ct.errCtrl, newStats)
			if retry {
				goto retry
			}
			if err != nil {
				return err
			}
			ct.skipDirs = append(ct.skipDirs, file.path)
		}
		return nil
	}
	return ct.copyFile(ctx, task, newStats)
}

func (ct *copyTask) mkDir(ctx context.Context, task *task.Task, file *fileStat, dstAbs string) error {
retry:
	// check task is canceled
	if task.Canceled() {
		return context.Canceled
	}
	err := os.MkdirAll(dstAbs, file.stat.Mode().Perm())
	if err != nil {
		retry, err := noticeFailedToCopyDir(ctx, task, ct.errCtrl, dstAbs, err)
		if retry {
			goto retry
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (ct *copyTask) copyFile(ctx context.Context, task *task.Task, stats *srcDstStat) (err error) {
	// update detail
	//
	// copying file, name: test.dat size: 1.127MB
	// src: C:\testdata\test.dat
	// dst: D:\test\test.dat
	_, srcFileName := filepath.Split(stats.srcAbs)
	srcSize := convert.ByteToString(uint64(stats.srcStat.Size()))
	const format = "copying file, name: %s size: %s\nsrc: %s\ndst: %s"
	ct.updateDetail(fmt.Sprintf(format, srcFileName, srcSize, stats.srcAbs, stats.dstAbs))

	// check dst file is exist
	if stats.dstStat != nil {
		if stats.dstStat.IsDir() {
			retry, err := noticeSameFileDir(ctx, task, ct.errCtrl, stats)
			if retry {
				return ct.retryCopyFile(ctx, task, stats)
			}
			ct.updateCurrent(stats.srcStat.Size(), true)
			return err
		}
		replace, err := noticeSameFile(ctx, task, ct.errCtrl, stats)
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
			retry, err = noticeFailedToCopy(ctx, task, ct.errCtrl, stats, err)
			if retry {
				err = ct.retryCopyFile(ctx, task, stats)
			} else if err == nil { // skipped
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
	var ok bool
	defer func() {
		_ = dstFile.Close()
		if !ok {
			_ = os.Remove(stats.dstAbs)
		}
	}()
	// update progress(actual size maybe changed)
	err = ct.updateActualProgress(srcFile, stats)
	if err != nil {
		return
	}
	// copy file
	copied, err = ioCopy(task, ct.ioCopyAdd, dstFile, srcFile)
	if err != nil {
		return
	}
	// sync
	err = dstFile.Sync()
	if err != nil {
		return
	}
	// set the modification time about the dst file
	modTime := stats.srcStat.ModTime()
	err = os.Chtimes(stats.dstAbs, modTime, modTime)
	if err != nil {
		return
	}
	ok = true
	return
}

func (ct *copyTask) updateActualProgress(srcFile *os.File, stats *srcDstStat) error {
	stat, err := srcFile.Stat()
	if err != nil {
		return err
	}
	// update total(must update in one operation)
	// total - old size + new size = total + (new size - old size)
	newSize := stat.Size()
	oldSize := stats.srcStat.Size()
	if newSize != oldSize {
		delta := newSize - oldSize
		ct.updateTotal(delta, true)
	}
	stats.srcStat = stat
	return nil
}

func (ct *copyTask) ioCopyAdd(delta int64) {
	ct.updateCurrent(delta, true)
}

// retryCopyFile will update src and dst file stat.
func (ct *copyTask) retryCopyFile(ctx context.Context, task *task.Task, stats *srcDstStat) error {
	srcStat, err := os.Stat(stats.srcAbs)
	if err != nil {
		return err
	}
	dstStat, err := stat(stats.dstAbs)
	if err != nil {
		return err
	}
	// update total(must update in one operation)
	// total - old size + new size = total + (new size - old size)
	newSize := srcStat.Size()
	oldSize := stats.srcStat.Size()
	if newSize != oldSize {
		delta := newSize - oldSize
		ct.updateTotal(delta, true)
	}
	stats.srcStat = srcStat
	stats.dstStat = dstStat
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
		ct.total.Add(ct.total, d)
	} else {
		ct.total.Sub(ct.total, d)
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
	case 0: // current == total
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
	return CopyWithContext(context.Background(), errCtrl, src, dst)
}

// CopyWithContext is used to create a copyTask with context.
func CopyWithContext(ctx context.Context, errCtrl ErrCtrl, src, dst string) error {
	ct := NewCopyTask(errCtrl, src, dst, nil)
	if done := ctx.Done(); done != nil {
		innerCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		go func() {
			defer func() {
				if r := recover(); r != nil {
					xpanic.Log(r, "CopyWithContext")
				}
			}()
			select {
			case <-ctx.Done():
				ct.Cancel()
			case <-innerCtx.Done():
			}
		}()
	}
	err := ct.Start()
	if err != nil {
		return err
	}
	// check progress
	progress := ct.Progress()
	if progress != "100%" {
		return errors.New("unexpected progress: " + progress)
	}
	return nil
}
