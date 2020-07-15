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
	"github.com/pkg/errors"

	"project/internal/module/task"
)

// moveTask implement task.Interface that is used to move source to destination.
// It can pause in progress and get current progress and detail information.
type moveTask struct {
	errCtrl ErrCtrl
	src     string
	dst     string
	stats   *SrcDstStat

	// store all dirs and files will move
	// key is path, value is a flag whether delete
	dirs     map[string]bool
	files    []*fileStat
	skipDirs []string

	// about progress and detail
	current *big.Float
	total   *big.Float
	detail  string
	rwm     sync.RWMutex
}

// NewMoveTask is used to create a move task that implement task.Interface.
func NewMoveTask(errCtrl ErrCtrl, src, dst string, callbacks fsm.Callbacks) *task.Task {
	mt := moveTask{
		errCtrl: errCtrl,
		src:     src,
		dst:     dst,
		current: big.NewFloat(0),
		total:   big.NewFloat(0),
	}
	return task.New(TaskNameMove, &mt, callbacks)
}

// Prepare will check source and destination path.
func (mt *moveTask) Prepare(context.Context) error {
	stats, err := checkSrcDstPath(mt.src, mt.dst)
	if err != nil {
		return err
	}
	mt.stats = stats
	return nil
}

func (mt *moveTask) Process(ctx context.Context, task *task.Task) error {
	if mt.stats.SrcIsFile {
		return mt.moveSrcFile(ctx, task)
	}
	return mt.moveSrcDir(ctx, task)
}

// moveSrcFile is used to move single file to a path.
//
// new path is a dir  and exist
// new path is a file and exist
// new path is a dir  and not exist
// new path is a file and not exist
func (mt *moveTask) moveSrcFile(ctx context.Context, task *task.Task) error {
	_, srcFileName := filepath.Split(mt.stats.SrcAbs)
	var (
		dstFileName string
		dstStat     os.FileInfo
	)
	if mt.stats.DstStat != nil { // dst is exists
		// moveFile will handle the same file, dir
		//
		// move "a.exe" -> "C:\ExistDir"
		// "a.exe" -> "C:\ExistDir\a.exe"
		if mt.stats.DstStat.IsDir() {
			dstFileName = filepath.Join(mt.stats.DstAbs, srcFileName)
			stat, err := stat(dstFileName)
			if err != nil {
				return err
			}
			dstStat = stat
		} else {
			dstFileName = mt.stats.DstAbs
			dstStat = mt.stats.DstStat
		}
	} else { // dst is doesn't exists
		dstRunes := []rune(mt.dst)
		last := string(dstRunes[len(dstRunes)-1])[0]
		if os.IsPathSeparator(last) { // is a directory path
			err := os.MkdirAll(mt.stats.DstAbs, 0750)
			if err != nil {
				return err
			}
			dstFileName = filepath.Join(mt.stats.DstAbs, srcFileName)
		} else { // is a file path
			dir, _ := filepath.Split(mt.stats.DstAbs)
			err := os.MkdirAll(dir, 0750)
			if err != nil {
				return err
			}
			dstFileName = mt.stats.DstAbs
		}
	}
	stats := &SrcDstStat{
		SrcAbs:  mt.stats.SrcAbs,
		DstAbs:  dstFileName,
		SrcStat: mt.stats.SrcStat,
		DstStat: dstStat,
	}
	// update progress
	mt.updateTotal(mt.stats.SrcStat.Size(), true)
	return mt.moveFile(ctx, task, stats)
}

// moveSrcDir is used to move directory to a path.
//
// move dir  C:\test -> D:\test2
// move file C:\test\file.dat -> C:\test2\file.dat
func (mt *moveTask) moveSrcDir(ctx context.Context, task *task.Task) error {
	err := mt.collectDirInfo(ctx, task)
	if err != nil {
		return errors.WithMessage(err, "failed to collect directory information")
	}
	return mt.moveDirFiles(ctx, task)
}

func (mt *moveTask) collectDirInfo(ctx context.Context, task *task.Task) error {
	mt.files = make([]*fileStat, 0, 64)
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
			ne := fmt.Errorf(format, srcAbs, mt.stats.SrcAbs, err)
			retry, ne := noticeFailedToCollect(ctx, task, mt.errCtrl, srcAbs, ne)
			if retry {
				walkFailed = true
				goto retry
			}
			if ne != nil {
				return ne
			}
			return filepath.SkipDir
		}
		mt.files = append(mt.files, &fileStat{
			path: srcAbs,
			stat: srcStat,
		})
		// update detail and total size
		if srcStat.IsDir() {
			// collecting directory information
			// path: C:\testdata\test
			const format = "collecting directory information\npath: %s"
			mt.updateDetail(fmt.Sprintf(format, srcAbs))
		} else {
			// collecting file information
			// path: C:\testdata\test
			const format = "collecting file information\npath: %s"
			mt.updateDetail(fmt.Sprintf(format, srcAbs))
			mt.updateTotal(srcStat.Size(), true)
		}
		return nil
	}
	return filepath.Walk(mt.stats.SrcAbs, walkFunc)
}

func (mt *moveTask) moveDirFiles(ctx context.Context, task *task.Task) error {
	// check root path, and make directory if target path is not exists
	// C:\test -> D:\test[exist]
	if mt.stats.DstStat == nil {
		err := os.MkdirAll(mt.stats.DstAbs, mt.stats.SrcStat.Mode().Perm())
		if err != nil {
			return errors.Wrap(err, "failed to create destination directory")
		}
	}
	// skip root directory
	// set fake progress for pass progress check
	if len(mt.files) == 0 {
		mt.current.SetUint64(1)
		mt.total.SetUint64(1)
		return nil
	}
	// must skip root path, otherwise will appear zero relative path
	for _, file := range mt.files[1:] {
		err := mt.moveDirFile(ctx, task, file)
		if err != nil {
			return errors.WithMessagef(err, "failed to copy file \"%s\"", file.path)
		}
	}
	return nil
}

func (mt *moveTask) moveDirFile(ctx context.Context, task *task.Task, file *fileStat) error {
	return nil
}

func (mt *moveTask) moveFile(ctx context.Context, task *task.Task, stats *SrcDstStat) error {
	if task.Canceled() {
		return context.Canceled
	}
	return nil
}

func (mt *moveTask) ioMove(ctx context.Context, task *task.Task, stats *SrcDstStat) (err error) {
	// check move file error, and maybe retry move file.
	var moved int64
	defer func() {
		if err != nil && err != context.Canceled {
			// reset current
			mt.updateCurrent(moved, false)
			var retry bool
			retry, err = noticeFailedToMove(ctx, task, mt.errCtrl, stats, err)
			if retry {
				err = mt.retryMoveFile(ctx, task, stats)
			} else if err == nil { // skipped
				mt.updateCurrent(stats.SrcStat.Size(), true)
			}
		}
	}()
	// use fast mode firstly
	enabled, err := mt.ioMoveFast(stats)
	if enabled {
		if err != nil {
			return
		}
		mt.updateCurrent(stats.SrcStat.Size(), true)
		return
	}
	// open src file
	srcFile, err := os.Open(stats.SrcAbs)
	if err != nil {
		return
	}
	defer func() { _ = srcFile.Close() }()
	// update progress(actual size maybe changed)
	err = mt.updateSrcFileStat(srcFile, stats)
	if err != nil {
		return
	}
	perm := stats.SrcStat.Mode().Perm()
	// open dst file
	dstFile, err := os.OpenFile(stats.DstAbs, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return
	}
	// if failed to move, delete dst file
	var ok bool
	defer func() {
		_ = dstFile.Close()
		if !ok {
			_ = os.Remove(stats.DstAbs)
		}
	}()
	// move file
	moved, err = ioCopy(task, mt.ioMoveAdd, dstFile, srcFile)
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

// ioMoveFast will use syscall to move file, it can move faster if two file in the same volume.
// Windows is finished, other platform need thinking.
func (mt *moveTask) ioMoveFast(stats *SrcDstStat) (bool, error) {
	srcVol := filepath.VolumeName(stats.SrcAbs)
	if srcVol == "" {
		return false, nil
	}
	dstVol := filepath.VolumeName(stats.DstAbs)
	if srcVol != dstVol {
		return false, nil
	}
	return true, os.Rename(stats.SrcAbs, stats.DstAbs)
}

func (mt *moveTask) updateSrcFileStat(srcFile *os.File, stats *SrcDstStat) error {
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
		mt.updateTotal(delta, true)
	}
	stats.SrcStat = srcStat
	return nil
}

func (mt *moveTask) ioMoveAdd(delta int64) {
	mt.updateCurrent(delta, true)
}

// retryMoveFile will update source and destination file stat.
func (mt *moveTask) retryMoveFile(ctx context.Context, task *task.Task, stats *SrcDstStat) error {
	dstStat, err := stat(stats.DstAbs)
	if err != nil {
		return err
	}
	stats.DstStat = dstStat
	return mt.moveFile(ctx, task, stats)
}

func (mt *moveTask) updateCurrent(delta int64, add bool) {
	mt.rwm.Lock()
	defer mt.rwm.Unlock()
	d := new(big.Float).SetInt64(delta)
	if add {
		mt.current.Add(mt.current, d)
	} else {
		mt.current.Sub(mt.current, d)
	}
}

func (mt *moveTask) updateTotal(delta int64, add bool) {
	mt.rwm.Lock()
	defer mt.rwm.Unlock()
	d := new(big.Float).SetInt64(delta)
	if add {
		mt.total.Add(mt.total, d)
	} else {
		mt.total.Sub(mt.total, d)
	}
}

// Progress is used to get progress about current move task.
//
// collect: "0%"
// move:    "15.22%|[current]/[total]"
// finish:  "100%"
func (mt *moveTask) Progress() string {
	mt.rwm.RLock()
	defer mt.rwm.RUnlock()
	// prevent / 0
	if mt.total.Cmp(zeroFloat) == 0 {
		return "0%"
	}
	switch mt.current.Cmp(mt.total) {
	case 0: // current == total
		return "100%"
	case 1: // current > total
		current := mt.current.Text('G', 64)
		total := mt.total.Text('G', 64)
		return fmt.Sprintf("err: current[%s] > total[%s]", current, total)
	}
	value := new(big.Float).Quo(mt.current, mt.total)
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
	current := mt.current.Text('G', 64)
	total := mt.total.Text('G', 64)
	return str + fmt.Sprintf("|[%s]/[%s]", current, total)
}

func (mt *moveTask) updateDetail(detail string) {
	mt.rwm.Lock()
	defer mt.rwm.Unlock()
	mt.detail = detail
}

func (mt *moveTask) Detail() string {
	mt.rwm.RLock()
	defer mt.rwm.RUnlock()
	return mt.detail
}

func (mt *moveTask) Clean() {}

// Move is used to create a moveTask to move file or directory.
func Move(errCtrl ErrCtrl, src, dst string) error {
	return MoveWithContext(context.Background(), errCtrl, src, dst)
}

// MoveWithContext is used to create a moveTask with context.
func MoveWithContext(ctx context.Context, errCtrl ErrCtrl, src, dst string) error {
	mt := NewMoveTask(errCtrl, src, dst, nil)
	return startTask(ctx, mt, "Move")
}
