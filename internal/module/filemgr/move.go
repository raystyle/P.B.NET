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
	"project/internal/xpanic"
)

// moveTask implement task.Interface that is used to move source to destination.
// It can pause in progress and get current progress and detail information.
type moveTask struct {
	errCtrl ErrCtrl
	src     string
	dst     string
	stats   *SrcDstStat

	// store all files will move
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
	return nil
}

func (mt *moveTask) moveFile(ctx context.Context, task *task.Task, stats *SrcDstStat) error {
	return nil
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
					xpanic.Log(r, "MoveWithContext")
				}
			}()
			select {
			case <-done:
				mt.Cancel()
			case <-finish:
			}
		}()
	}
	err := mt.Start()
	if err != nil {
		return err
	}
	// check progress
	progress := mt.Progress()
	if progress != "100%" {
		return errors.New("unexpected progress: " + progress)
	}
	return nil
}
