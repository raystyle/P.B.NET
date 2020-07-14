package filemgr

import (
	"context"
	"fmt"
	"math/big"
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

// Prepare will check src and dst path.
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
	return nil
}

// moveSrcDir is used to move directory to a path.
//
// move dir  C:\test -> D:\test2
// move file C:\test\file.dat -> C:\test2\file.dat
func (mt *moveTask) moveSrcDir(ctx context.Context, task *task.Task) error {
	return nil
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
