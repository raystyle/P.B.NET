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
	stats   *SrcDstStat

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
	if ct.stats.SrcIsFile {
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
	_, srcFileName := filepath.Split(ct.stats.SrcAbs)
	var (
		dstFileName string
		dstStat     os.FileInfo
	)
	if ct.stats.DstStat != nil { // dst is exists
		// copyFile will handle the same file, dir
		//
		// copy "a.exe" -> "C:\ExistDir"
		// "a.exe" -> "C:\ExistDir\a.exe"
		if ct.stats.DstStat.IsDir() {
			dstFileName = filepath.Join(ct.stats.DstAbs, srcFileName)
			stat, err := stat(dstFileName)
			if err != nil {
				return err
			}
			dstStat = stat
		} else {
			dstFileName = ct.stats.DstAbs
			dstStat = ct.stats.DstStat
		}
	} else { // dst is doesn't exists
		dstRunes := []rune(ct.dst)
		last := string(dstRunes[len(dstRunes)-1])[0]
		if os.IsPathSeparator(last) { // is a directory path
			err := os.MkdirAll(ct.stats.DstAbs, 0750)
			if err != nil {
				return err
			}
			dstFileName = filepath.Join(ct.stats.DstAbs, srcFileName)
		} else { // is a file path
			dir, _ := filepath.Split(ct.stats.DstAbs)
			err := os.MkdirAll(dir, 0750)
			if err != nil {
				return err
			}
			dstFileName = ct.stats.DstAbs
		}
	}
	stats := &SrcDstStat{
		SrcAbs:  ct.stats.SrcAbs,
		DstAbs:  dstFileName,
		SrcStat: ct.stats.SrcStat,
		DstStat: dstStat,
	}
	// update progress
	ct.updateTotal(ct.stats.SrcStat.Size(), true)
	return ct.copyFile(ctx, task, stats)
}

// copy C:\test -> D:\test2
// -- copy C:\test\file.dat -> C:\test2\file.dat
func (ct *copyTask) copySrcDir(ctx context.Context, task *task.Task) error {
	err := ct.collectDirInfo(ctx, task)
	if err != nil {
		return errors.WithMessage(err, "failed to collect directory information")
	}
	return ct.copyDirFiles(ctx, task)
}

// collect will collect directory information for calculate total size.
func (ct *copyTask) collectDirInfo(ctx context.Context, task *task.Task) error {
	ct.files = make([]*fileStat, 0, 64)
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
			ne := fmt.Errorf(format, srcAbs, ct.stats.SrcAbs, err)
			retry, ne := noticeFailedToCollect(ctx, task, ct.errCtrl, srcAbs, ne)
			if retry {
				walkFailed = true
				goto retry
			}
			if ne != nil {
				return ne
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
			// collecting file information
			// path: C:\testdata\test
			const format = "collecting file information\npath: %s"
			ct.updateDetail(fmt.Sprintf(format, srcAbs))
			ct.updateTotal(srcStat.Size(), true)
		}
		return nil
	}
	return filepath.Walk(ct.stats.SrcAbs, walkFunc)
}

func (ct *copyTask) copyDirFiles(ctx context.Context, task *task.Task) error {
	// check root path, and make directory if target path is not exists
	// C:\test -> D:\test[exist]
	if ct.stats.DstStat == nil {
		err := os.MkdirAll(ct.stats.DstAbs, ct.stats.SrcStat.Mode().Perm())
		if err != nil {
			return errors.Wrap(err, "failed to create destination directory")
		}
	}
	// skip root directory
	// set fake progress for pass progress check
	if len(ct.files) == 0 {
		ct.current.SetUint64(1)
		ct.total.SetUint64(1)
		return nil
	}
	// must skip root path, otherwise will appear zero relative path
	for _, file := range ct.files[1:] {
		err := ct.copyDirFile(ctx, task, file)
		if err != nil {
			return errors.WithMessagef(err, "failed to copy file \"%s\"", file.path)
		}
	}
	return nil
}

func (ct *copyTask) copyDirFile(ctx context.Context, task *task.Task, file *fileStat) error {
	// skip file if it in skipped directories
	for i := 0; i < len(ct.skipDirs); i++ {
		if strings.HasPrefix(file.path, ct.skipDirs[i]) {
			ct.updateCurrent(file.stat.Size(), true)
			return nil
		}
	}
	// C:\test\a.exe -> a.exe
	// C:\test\dir\a.exe -> dir\a.exe
	relativePath := strings.ReplaceAll(file.path, ct.stats.SrcAbs, "")
	relativePath = string([]rune(relativePath)[1:]) // remove the first "\" or "/"
	dstAbs := filepath.Join(ct.stats.DstAbs, relativePath)
retry:
	// check task is canceled
	if task.Canceled() {
		return context.Canceled
	}
	// dstStat maybe updated
	dstStat, err := stat(dstAbs)
	if err != nil {
		retry, ne := noticeFailedToCopyDir(ctx, task, ct.errCtrl, dstAbs, err)
		if retry {
			goto retry
		}
		if ne != nil {
			return ne
		}
		if file.stat.IsDir() {
			ct.skipDirs = append(ct.skipDirs, file.path)
			return nil
		}
		ct.updateCurrent(file.stat.Size(), true)
		return nil
	}
	newStats := &SrcDstStat{
		SrcAbs:  file.path,
		DstAbs:  dstAbs,
		SrcStat: file.stat,
		DstStat: dstStat,
	}
	if file.stat.IsDir() {
		if dstStat == nil {
			return ct.mkdir(ctx, task, file, dstAbs)
		}
		if !dstStat.IsDir() {
			retry, ne := noticeSameDirFile(ctx, task, ct.errCtrl, newStats)
			if retry {
				goto retry
			}
			if ne != nil {
				return ne
			}
			ct.skipDirs = append(ct.skipDirs, file.path)
		}
		return nil
	}
	return ct.copyFile(ctx, task, newStats)
}

func (ct *copyTask) mkdir(ctx context.Context, task *task.Task, file *fileStat, dstAbs string) error {
retry:
	// check task is canceled
	if task.Canceled() {
		return context.Canceled
	}
	err := os.MkdirAll(dstAbs, file.stat.Mode().Perm())
	if err != nil {
		retry, ne := noticeFailedToCopyDir(ctx, task, ct.errCtrl, dstAbs, err)
		if retry {
			goto retry
		}
		if ne != nil {
			return ne
		}
		ct.skipDirs = append(ct.skipDirs, file.path)
	}
	return nil
}

func (ct *copyTask) copyFile(ctx context.Context, task *task.Task, stats *SrcDstStat) error {
	if task.Canceled() {
		return context.Canceled
	}
	// check src file is become directory
	srcStat, err := os.Stat(stats.SrcAbs)
	if err != nil {
		retry, ne := noticeFailedToCopy(ctx, task, ct.errCtrl, stats, err)
		if retry {
			return ct.retryCopyFile(ctx, task, stats)
		}
		if ne != nil {
			return ne
		}
		ct.updateCurrent(stats.SrcStat.Size(), true)
		return nil
	}
	if srcStat.IsDir() {
		err = os.MkdirAll(stats.DstAbs, srcStat.Mode().Perm())
		if err != nil {
			retry, ne := noticeFailedToCopy(ctx, task, ct.errCtrl, stats, err)
			if retry {
				return ct.retryCopyFile(ctx, task, stats)
			}
			if ne != nil {
				return ne
			}
			ct.skipDirs = append(ct.skipDirs, stats.SrcAbs)
		}
		ct.updateCurrent(stats.SrcStat.Size(), true)
		return nil
	}
	// update current task detail, output:
	//   copying file, name: test.dat size: 1.127MB
	//   src: C:\testdata\test.dat
	//   dst: D:\test\test.dat
	_, srcFileName := filepath.Split(stats.SrcAbs)
	srcSize := convert.ByteToString(uint64(stats.SrcStat.Size()))
	const format = "copying file, name: %s size: %s\nsrc: %s\ndst: %s"
	ct.updateDetail(fmt.Sprintf(format, srcFileName, srcSize, stats.SrcAbs, stats.DstAbs))
	// check dst file is exist
	if stats.DstStat != nil {
		if stats.DstStat.IsDir() {
			retry, err := noticeSameFileDir(ctx, task, ct.errCtrl, stats)
			if retry {
				return ct.retryCopyFile(ctx, task, stats)
			}
			ct.updateCurrent(stats.SrcStat.Size(), true)
			return err
		}
		replace, err := noticeSameFile(ctx, task, ct.errCtrl, stats)
		if !replace {
			ct.updateCurrent(stats.SrcStat.Size(), true)
			return err
		}
	}
	return ct.ioCopy(ctx, task, stats)
}

func (ct *copyTask) ioCopy(ctx context.Context, task *task.Task, stats *SrcDstStat) (err error) {
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
				ct.updateCurrent(stats.SrcStat.Size(), true)
			}
		}
	}()
	// open src file
	srcFile, err := os.Open(stats.SrcAbs)
	if err != nil {
		return
	}
	defer func() { _ = srcFile.Close() }()
	// update progress(actual size maybe changed)
	err = ct.updateSrcFileStat(srcFile, stats)
	if err != nil {
		return
	}
	perm := stats.SrcStat.Mode().Perm()
	// open dst file
	dstFile, err := os.OpenFile(stats.DstAbs, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return
	}
	// if failed to copy, delete dst file
	var ok bool
	defer func() {
		_ = dstFile.Close()
		if !ok {
			_ = os.Remove(stats.DstAbs)
		}
	}()
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
	modTime := stats.SrcStat.ModTime()
	err = os.Chtimes(stats.DstAbs, modTime, modTime)
	if err != nil {
		return
	}
	ok = true
	return
}

func (ct *copyTask) updateSrcFileStat(srcFile *os.File, stats *SrcDstStat) error {
	srcStat, err := srcFile.Stat()
	if err != nil {
		return err
	}
	// update total(must update in one operation)
	// total - old size + new size = total + (new size - old size)
	newSize := srcStat.Size()
	oldSize := stats.SrcStat.Size()
	if newSize != oldSize {
		delta := newSize - oldSize
		ct.updateTotal(delta, true)
	}
	stats.SrcStat = srcStat
	return nil
}

func (ct *copyTask) ioCopyAdd(delta int64) {
	ct.updateCurrent(delta, true)
}

// retryCopyFile will update src and dst file stat.
func (ct *copyTask) retryCopyFile(ctx context.Context, task *task.Task, stats *SrcDstStat) error {
	dstStat, err := stat(stats.DstAbs)
	if err != nil {
		return err
	}
	stats.DstStat = dstStat
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
		// if ctx is canceled
		select {
		case <-done:
			return ctx.Err()
		default:
		}
		// start a goroutine to watch ctx
		finish := make(chan struct{})
		defer close(finish)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					xpanic.Log(r, "CopyWithContext")
				}
			}()
			select {
			case <-done:
				ct.Cancel()
			case <-finish:
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
