package filemgr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/looplab/fsm"

	"project/internal/convert"
	"project/internal/module/task"
	"project/internal/xio"
)

// Copy is used to copy file or directory from source path to destination path,
// if the target file is exist, will call exist function and replace it if replace
// function return true.
func Copy(ec ErrCtrl, src, dst string) error {
	return copyWithContext(context.Background(), ec, src, dst)
}

// CopyWithContext is used to copy file or directory from source path to destination
// path with context.
func CopyWithContext(ctx context.Context, ec ErrCtrl, src, dst string) error {
	return copyWithContext(ctx, ec, src, dst)
}

func copyWithContext(ctx context.Context, ec ErrCtrl, src, dst string) error {
	stats, err := checkSrcDstPath(src, dst)
	if err != nil {
		return err
	}
	// copy file to a path
	if stats.srcIsFile {
		stats.dst = dst
		return copySrcFile(ctx, ec, stats)
	}
	return copySrcDir(ctx, ec, stats)
}

// copySrcFile is used to copy single file to a path.
//
// new path is a dir  and exist
// new path is a file and exist
// new path is a dir  and not exist
// new path is a file and not exist
func copySrcFile(ctx context.Context, ec ErrCtrl, stats *srcDstStat) error {
	_, srcFileName := filepath.Split(stats.srcAbs)
	var (
		dstFileName string
		dstStat     os.FileInfo
	)
	if stats.dstStat != nil { // dst is exists
		// copyFile will handle the same file, dir
		//
		// copy "a.exe" -> "C:\ExistDir"
		// "a.exe" -> "C:\ExistDir\a.exe"
		if stats.dstStat.IsDir() {
			dstFileName = filepath.Join(stats.dstAbs, srcFileName)
			stat, err := stat(dstFileName)
			if err != nil {
				return err
			}
			dstStat = stat
		} else {
			dstFileName = stats.dstAbs
			dstStat = stats.dstStat
		}
	} else { // dst is doesn't exists
		last := stats.dst[len(stats.dst)-1]
		if os.IsPathSeparator(last) { // is a directory path
			err := os.MkdirAll(stats.dstAbs, 0750)
			if err != nil {
				return err
			}
			dstFileName = filepath.Join(stats.dstAbs, srcFileName)
		} else { // is a file path
			dir, _ := filepath.Split(stats.dstAbs)
			err := os.MkdirAll(dir, 0750)
			if err != nil {
				return err
			}
			dstFileName = stats.dstAbs
		}
	}
	newStats := &srcDstStat{
		srcAbs:  stats.srcAbs,
		dstAbs:  dstFileName,
		srcStat: stats.srcStat,
		dstStat: dstStat,
	}
	return copyFile(ctx, ec, newStats)
}

// copy C:\test -> D:\test2
// -- copy C:\test\file.dat -> C:\test2\file.dat
func copySrcDir(ctx context.Context, ec ErrCtrl, stats *srcDstStat) error {
	var skippedDirs []string // used to store skipped directories
	walkFunc := func(srcAbs string, srcStat os.FileInfo, err error) error {
		// check is canceled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		// check walk error
		if err != nil {
			return fmt.Errorf("failed to walk \"%s\": %s", srcAbs, err)
		}
		// check is root path, and make directory if target path is not exists
		// C:\test -> D:\test[exist]
		if srcAbs == stats.srcAbs {
			if stats.dstStat == nil {
				return os.MkdirAll(stats.dstAbs, stats.srcStat.Mode().Perm())
			}
			return nil
		}
		// skip file in skipped directories
		for i := 0; i < len(skippedDirs); i++ {
			if strings.Contains(srcAbs, skippedDirs[i]) {
				return nil
			}
		}
		// C:\test\a.exe -> a.exe
		// C:\test\dir\a.exe -> dir\a.exe
		relativePath := strings.ReplaceAll(srcAbs, stats.srcAbs, "")
		// remove the first "\" or "/"
		relativePath = string([]rune(relativePath)[1:])
		dstAbs := filepath.Join(stats.dstAbs, relativePath)
	retry: // dstStat maybe updated
		dstStat, err := stat(dstAbs)
		if err != nil {
			return err
		}
		newStats := &srcDstStat{
			srcAbs:  srcAbs,
			dstAbs:  dstAbs,
			srcStat: srcStat,
			dstStat: dstStat,
		}
		if srcStat.IsDir() {
			if dstStat == nil {
				return os.MkdirAll(dstAbs, srcStat.Mode().Perm())
			}
			if !dstStat.IsDir() {
				retry, err := noticeSameDirFile(ec, newStats)
				if retry {
					goto retry
				}
				if err != nil {
					return err
				}
				skippedDirs = append(skippedDirs, srcAbs)
			}
			return nil
		}
		return copyFile(ctx, ec, newStats)
	}
	return filepath.Walk(stats.srcAbs, walkFunc)
}

// dst abs path doesn't have to exist, two abs path are all file.
func copyFile(ctx context.Context, ec ErrCtrl, stats *srcDstStat) (err error) {
	// check dst file is exist
	if stats.dstStat != nil {
		if stats.dstStat.IsDir() {
			retry, err := noticeSameFileDir(ec, stats)
			if retry {
				return retryCopyFile(ctx, ec, stats)
			}
			return err
		}
		replace, err := noticeSameFile(ec, stats)
		if !replace {
			return err
		}
	}
	// check copy file error, and maybe retry copy file.
	defer func() {
		if err != nil && err != context.Canceled {
			var retry bool
			retry, err = noticeFailedToCopy(ec, stats, err)
			if retry {
				err = retryCopyFile(ctx, ec, stats)
			}
		}
	}()
	// src file
	srcFile, err := os.Open(stats.srcAbs)
	if err != nil {
		return
	}
	defer func() { _ = srcFile.Close() }()
	// dst file
	perm := stats.srcStat.Mode().Perm()
	dstFile, err := os.OpenFile(stats.dstAbs, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return
	}
	defer func() { _ = dstFile.Close() }()
	// copy file
	_, err = xio.CopyWithContext(ctx, dstFile, srcFile)
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

// retryCopyFile will update src and dst file stat.
func retryCopyFile(ctx context.Context, ec ErrCtrl, stats *srcDstStat) error {
	var err error
	stats.srcStat, err = os.Stat(stats.srcAbs)
	if err != nil {
		return err
	}
	stats.dstStat, err = stat(stats.dstAbs)
	if err != nil {
		return err
	}
	return copyFile(ctx, ec, stats)
}

// copyTask implement task.Interface that is used to copy src to dst directory.
// It can pause in progress and get current progress and detail information.
type copyTask struct {
	errCtrl ErrCtrl
	src     string
	dst     string
	stats   *srcDstStat

	progress float32
	detail   string
	rwm      sync.RWMutex
}

// NewCopyTask is used to create a copy task that implement task.Interface.
func NewCopyTask(errCtrl ErrCtrl, src, dst string, callbacks fsm.Callbacks) *task.Task {
	ct := copyTask{
		errCtrl: errCtrl,
		src:     src,
		dst:     dst,
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
		return ct.copySingleFile(ctx, task)
	}
	return nil
}

func (ct *copyTask) copySingleFile(ctx context.Context, task *task.Task) error {
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
	return ct.copyFile(ctx, task, stats)
}

// collect will collect directory information.
func (ct *copyTask) collect() error {
	return nil
}

func (ct *copyTask) copyFile(ctx context.Context, task *task.Task, stats *srcDstStat) error {

	// update detail information
	const detailFormat = "size: %s path: %s -> %s"
	srcSize := convert.ByteToString(uint64(stats.srcStat.Size()))
	ct.updateDetail(fmt.Sprintf(detailFormat, srcSize, stats.srcAbs, stats.dstAbs))

	//

	return nil
}

func (ct *copyTask) updateProgress(progress float32) {
	ct.rwm.Lock()
	defer ct.rwm.Unlock()
	ct.progress = progress
}

func (ct *copyTask) updateDetail(detail string) {
	ct.rwm.Lock()
	defer ct.rwm.Unlock()
	ct.detail = detail
}

func (ct *copyTask) Progress() float32 {
	ct.rwm.RLock()
	defer ct.rwm.RUnlock()
	return ct.progress
}

func (ct *copyTask) Detail() string {
	ct.rwm.RLock()
	defer ct.rwm.RUnlock()
	return ct.detail
}

func (ct *copyTask) Clean() {}
