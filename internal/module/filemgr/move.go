package filemgr

import (
	"context"
	"fmt"
	"io"
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
	"project/internal/module/task"
	"project/internal/xpanic"
)

// moveTask implement task.Interface that is used to move source to destination.
// It can pause in progress and get current progress and detail information.
type moveTask struct {
	errCtrl  ErrCtrl
	dst      string
	paths    []string // absolute path that will be compressed
	pathsLen int

	// TODO delete these
	stats *SrcDstStat
	root  *file

	dstStat  os.FileInfo
	basePath string           // for filepath.Rel() in Process
	roots    []*file          // store all directories and files will move
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

// NewMoveTask is used to create a move task that implement task.Interface.
// Files must in the same directory.
func NewMoveTask(errCtrl ErrCtrl, callbacks fsm.Callbacks, dst string, paths ...string) *task.Task {
	mt := moveTask{
		errCtrl:    errCtrl,
		dst:        dst,
		paths:      paths,
		pathsLen:   len(paths),
		current:    big.NewFloat(0),
		total:      big.NewFloat(0),
		stopSignal: make(chan struct{}),
	}
	return task.New(TaskNameMove, &mt, callbacks)
}

// Prepare will check source and destination path.
func (mt *moveTask) Prepare(context.Context) error {
	// check paths
	if mt.pathsLen == 0 {
		return errors.New("empty path")
	}
	// check destination is not exist or a file.
	dstAbs, err := filepath.Abs(mt.dst)
	if err != nil {
		return errors.Wrap(err, "failed to get absolute file path")
	}
	dstStat, err := stat(dstAbs)
	if err != nil {
		return err
	}
	if dstStat != nil && !dstStat.IsDir() {
		return errors.Errorf("destination path \"%s\" is a file", dstAbs)
	}
	// check paths is valid
	// check paths is valid
	mt.basePath, err = validatePaths(mt.paths)
	if err != nil {
		return err
	}
	mt.dst = dstAbs
	mt.dstStat = dstStat
	mt.roots = make([]*file, mt.pathsLen)

	stats, err := checkSrcDstPath(mt.paths[0], mt.dst)
	if err != nil {
		return err
	}
	mt.stats = stats
	go mt.watcher()
	return nil
}

func (mt *moveTask) Process(ctx context.Context, task *task.Task) error {
	defer mt.updateDetail("finished")
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
	srcFileName := filepath.Base(mt.stats.SrcAbs)
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
			s, err := stat(dstFileName)
			if err != nil {
				return err
			}
			dstStat = s
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
			dir := filepath.Dir(mt.stats.DstAbs)
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
	_, err := mt.moveFile(ctx, task, stats)
	return err
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
	return mt.moveRoot(ctx, task)
}

// collectDirInfo will collect directory information for calculate total size.
func (mt *moveTask) collectDirInfo(ctx context.Context, task *task.Task) error {
	var (
		cDir  string // current directory
		cFile *file  // current file
	)
	walkFunc := func(srcAbs string, srcStat os.FileInfo, err error) error {
		if err != nil {
			ps := noticePs{
				ctx:     ctx,
				task:    task,
				errCtrl: mt.errCtrl,
			}
			const format = "failed to walk \"%s\" in \"%s\": %s"
			err = fmt.Errorf(format, srcAbs, mt.stats.SrcAbs, err)
			skip, ne := noticeFailedToCollect(&ps, srcAbs, err)
			if skip {
				return filepath.SkipDir
			}
			return ne
		}
		if task.Canceled() {
			return context.Canceled
		}
		f := &file{
			path: srcAbs,
			stat: srcStat,
		}
		// check is root directory
		if mt.root == nil {
			// initialize task structure
			mt.root = f
			mt.dirs = make(map[string]*file)
			mt.dirs[srcAbs] = f
			// set current data
			cDir = srcAbs
			cFile = f
			return nil
		}
		// update detail and total size
		dir := filepath.Dir(srcAbs)
		if dir != cDir {
			cDir = dir
			cFile = mt.dirs[dir]
		}
		cFile.files = append(cFile.files, f)
		if srcStat.IsDir() {
			cDir = srcAbs
			cFile = f
			mt.dirs[srcAbs] = f
			// collecting directory information
			// path: C:\testdata\test
			mt.updateDetail("collect directory information\npath: " + srcAbs)
			return nil
		}
		// collecting file information
		// path: C:\testdata\test
		mt.updateDetail("collect file information\npath: " + srcAbs)
		mt.updateTotal(srcStat.Size(), true)
		return nil
	}
	return filepath.Walk(mt.stats.SrcAbs, walkFunc)
}

func (mt *moveTask) moveRoot(ctx context.Context, task *task.Task) error {
	// skip root directory
	// set fake progress for pass progress check
	if mt.root == nil {
		mt.rwm.Lock()
		defer mt.rwm.Unlock()
		mt.current.SetUint64(1)
		mt.total.SetUint64(1)
		return nil
	}
	// check root path, and make directory if target path is not exists
	// C:\test -> D:\test[exist]
	if mt.stats.DstStat == nil {
		err := os.MkdirAll(mt.stats.DstAbs, mt.stats.SrcStat.Mode().Perm())
		if err != nil {
			return errors.Wrap(err, "failed to create destination directory")
		}
	}
	_, err := mt.moveDir(ctx, task, mt.root)
	if err != nil {
		return errors.WithMessage(err, "failed to move directory")
	}
	return nil
}

// returned bool is skipped this file.
func (mt *moveTask) moveDir(ctx context.Context, task *task.Task, dir *file) (bool, error) {
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
			err = mt.mkdir(ctx, task, file)
			if err != nil {
				return false, err
			}
			skip, err = mt.moveDir(ctx, task, file)
		} else {
			skip, err = mt.moveDirFile(ctx, task, file)
		}
		if err != nil {
			return false, err
		}
		if skip && !skipped {
			skipped = true
		}
	}
	// if move all files successfully, remove the directory
	if !skipped {
		return false, os.Remove(dir.path)
	}
	return true, nil
}

func (mt *moveTask) mkdir(ctx context.Context, task *task.Task, dir *file) error {
	// skip dir if it in skipped directories
	for i := 0; i < len(mt.skipDirs); i++ {
		if strings.HasPrefix(dir.path, mt.skipDirs[i]) {
			return nil
		}
	}
	// calculate destination absolute path
	// C:\test\a.exe -> a.exe
	// C:\test\dir\a.exe -> dir\a.exe
	relativePath := strings.Replace(dir.path, mt.stats.SrcAbs, "", 1)
	relativePath = string([]rune(relativePath)[1:]) // remove the first "\" or "/"
	dstAbs := filepath.Join(mt.stats.DstAbs, relativePath)
retry:
	// check task is canceled
	if task.Canceled() {
		return context.Canceled
	}
	// check destination directory is exist
	dstStat, err := stat(dstAbs)
	if err != nil {
		ps := noticePs{
			ctx:     ctx,
			task:    task,
			errCtrl: mt.errCtrl,
		}
		retry, ne := noticeFailedToMoveDir(&ps, nil, err) // TODO stats
		if retry {
			goto retry
		}
		if ne != nil {
			return ne
		}
		mt.skipDirs = append(mt.skipDirs, dir.path)
		return nil
	}
	if dstStat != nil {
		if dstStat.IsDir() {
			return nil
		}
		ps := noticePs{
			ctx:     ctx,
			task:    task,
			errCtrl: mt.errCtrl,
		}
		stats := SrcDstStat{
			SrcAbs:  dir.path,
			DstAbs:  dstAbs,
			SrcStat: dir.stat,
			DstStat: dstStat,
		}
		retry, ne := noticeSameDirFile(&ps, &stats)
		if retry {
			goto retry
		}
		if ne != nil {
			return ne
		}
		mt.skipDirs = append(mt.skipDirs, dir.path)
		return nil
	}
	err = os.Mkdir(dstAbs, dir.stat.Mode().Perm())
	if err != nil {
		ps := noticePs{
			ctx:     ctx,
			task:    task,
			errCtrl: mt.errCtrl,
		}
		retry, ne := noticeFailedToMoveDir(&ps, nil, err) // TODO stats
		if retry {
			goto retry
		}
		if ne != nil {
			return ne
		}
		mt.skipDirs = append(mt.skipDirs, dir.path)
	}
	return nil
}

// returned bool is skipped this file.
func (mt *moveTask) moveDirFile(ctx context.Context, task *task.Task, file *file) (bool, error) {
	// skip file if it in skipped directories
	for i := 0; i < len(mt.skipDirs); i++ {
		if strings.HasPrefix(file.path, mt.skipDirs[i]) {
			mt.updateCurrent(file.stat.Size(), true)
			return true, nil
		}
	}
	// calculate destination absolute path
	// C:\test\a.exe -> a.exe
	// C:\test\dir\a.exe -> dir\a.exe
	relativePath := strings.Replace(file.path, mt.stats.SrcAbs, "", 1)
	relativePath = string([]rune(relativePath)[1:]) // remove the first "\" or "/"
	dstAbs := filepath.Join(mt.stats.DstAbs, relativePath)
retry:
	// check task is canceled
	if task.Canceled() {
		return false, context.Canceled
	}
	// dstStat maybe updated
	dstStat, err := stat(dstAbs)
	if err != nil {
		ps := noticePs{
			ctx:     ctx,
			task:    task,
			errCtrl: mt.errCtrl,
		}
		retry, ne := noticeFailedToMoveDir(&ps, nil, err) // TODO stats
		if retry {
			goto retry
		}
		if ne != nil {
			return false, ne
		}
		mt.updateCurrent(file.stat.Size(), true)
		return true, nil
	}
	stats := &SrcDstStat{
		SrcAbs:  file.path,
		DstAbs:  dstAbs,
		SrcStat: file.stat,
		DstStat: dstStat,
	}
	return mt.moveFile(ctx, task, stats)
}

// returned bool is skipped this file.
func (mt *moveTask) moveFile(ctx context.Context, task *task.Task, stats *SrcDstStat) (bool, error) {
	if task.Canceled() {
		return false, context.Canceled
	}
	// check src file is become directory
	srcStat, err := os.Stat(stats.SrcAbs)
	if err != nil {
		ps := noticePs{
			ctx:     ctx,
			task:    task,
			errCtrl: mt.errCtrl,
		}
		retry, ne := noticeFailedToMove(&ps, stats, err)
		if retry {
			return mt.retryMoveFile(ctx, task, stats)
		}
		if ne != nil {
			return false, ne
		}
		mt.updateCurrent(stats.SrcStat.Size(), true)
		return true, nil
	}
	if srcStat.IsDir() {
		ps := noticePs{
			ctx:     ctx,
			task:    task,
			errCtrl: mt.errCtrl,
		}
		err = errors.New("source file become directory")
		retry, ne := noticeFailedToMove(&ps, stats, err)
		if retry {
			return mt.retryMoveFile(ctx, task, stats)
		}
		if ne != nil {
			return false, ne
		}
		mt.updateCurrent(stats.SrcStat.Size(), true)
		return true, nil
	}
	return mt.ioMove(ctx, task, stats)
}

func (mt *moveTask) ioMove(ctx context.Context, task *task.Task, stats *SrcDstStat) (skipped bool, err error) {
	// check move file error, and maybe retry move file.
	var moved int64
	defer func() {
		if err != nil && err != context.Canceled {
			// reset current progress
			mt.updateCurrent(moved, false)
			ps := noticePs{
				ctx:     ctx,
				task:    task,
				errCtrl: mt.errCtrl,
			}
			var retry bool
			retry, err = noticeFailedToMove(&ps, stats, err)
			if retry {
				skipped, err = mt.retryMoveFile(ctx, task, stats)
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
	return
	// return mt.ioMoveCommon(task, stats, &moved)
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

func (mt *moveTask) ioMoveCommon(task *task.Task, stats *SrcDstStat, moved *int64) error {
	// open src file
	srcFile, err := os.Open(stats.SrcAbs)
	if err != nil {
		return err
	}
	var ok bool
	defer func() {
		_ = srcFile.Close()
		if ok {
			err = os.Remove(stats.SrcAbs)
		}
	}()
	// update progress(actual size maybe changed)
	err = mt.updateSrcFileStat(srcFile, stats)
	if err != nil {
		return err
	}
	perm := stats.SrcStat.Mode().Perm()
	// open dst file
	dstFile, err := os.OpenFile(stats.DstAbs, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm) // #nosec
	if err != nil {
		return err
	}
	// if failed to move, delete dst file
	defer func() {
		_ = dstFile.Close()
		if !ok {
			_ = os.Remove(stats.DstAbs)
		}
	}()
	// move file
	lr := io.LimitReader(srcFile, stats.SrcStat.Size())
	*moved, err = ioCopy(task, mt.ioMoveAdd, dstFile, lr)
	if err != nil {
		return err
	}
	// sync
	err = dstFile.Sync()
	if err != nil {
		return err
	}
	// set the modification time about the dst file
	modTime := stats.SrcStat.ModTime()
	err = os.Chtimes(stats.DstAbs, modTime, modTime)
	if err != nil {
		return err
	}
	ok = true
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
func (mt *moveTask) retryMoveFile(ctx context.Context, task *task.Task, stats *SrcDstStat) (bool, error) {
	dstStat, err := stat(stats.DstAbs)
	if err != nil {
		return false, err
	}
	stats.DstStat = dstStat
	return mt.moveFile(ctx, task, stats)
}

// Progress is used to get progress about current move task.
//
// collect: "0%"
// move:    "15.22%|current/total|128 MB/s"
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
		return fmt.Sprintf("error: current[%s] > total[%s]", current, total)
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
		return fmt.Sprintf("error: %s", err)
	}
	// 0.9999 -> 99.99%
	progress := strconv.FormatFloat(result*100, 'f', -1, 64) + "%"
	offset := strings.Index(progress, ".")
	if offset != -1 {
		if len(progress[offset+1:]) > 2 {
			progress = progress[:offset+3]
		}
	}
	// progress|current/total|speed
	current := mt.current.Text('G', 64)
	total := mt.total.Text('G', 64)
	speed := convert.FormatByte(mt.speed)
	return fmt.Sprintf("%s%%|%s/%s|%s/s", progress, current, total, speed)
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

// Detail is used to get detail about move task.
//
// collect dir info:
//   collect directory information
//   path: C:\testdata\test
//
// move file:
//   move file, name: test.dat size: 1.127MB
//   src: C:\testdata\test.dat
//   dst: D:\test\test.dat
func (mt *moveTask) Detail() string {
	mt.rwm.RLock()
	defer mt.rwm.RUnlock()
	return mt.detail
}

func (mt *moveTask) updateDetail(detail string) {
	mt.rwm.Lock()
	defer mt.rwm.Unlock()
	mt.detail = detail
}

// watcher is used to calculate current move speed.
func (mt *moveTask) watcher() {
	defer func() {
		if r := recover(); r != nil {
			xpanic.Log(r, "moveTask.watcher")
		}
	}()
	ticker := time.NewTicker(time.Second / time.Duration(len(mt.speeds)))
	defer ticker.Stop()
	current := new(big.Float)
	index := -1
	for {
		select {
		case <-ticker.C:
			index++
			if index >= len(mt.speeds) {
				index = 0
			}
			mt.watchSpeed(current, index)
		case <-mt.stopSignal:
			return
		}
	}
}

func (mt *moveTask) watchSpeed(current *big.Float, index int) {
	mt.rwm.Lock()
	defer mt.rwm.Unlock()
	delta := new(big.Float).Sub(mt.current, current)
	current.Add(current, delta)
	// update speed
	mt.speeds[index], _ = delta.Uint64()
	if mt.full {
		mt.speed = 0
		for i := 0; i < len(mt.speeds); i++ {
			mt.speed += mt.speeds[i]
		}
		return
	}
	if index == len(mt.speeds)-1 {
		mt.full = true
	}
	// calculate average speed
	var speed float64 // current speed
	for i := 0; i < index+1; i++ {
		speed += float64(mt.speeds[i])
	}
	mt.speed = uint64(speed / float64(index+1) * float64(len(mt.speeds)))
}

// Clean is used to send stop signal to watcher.
func (mt *moveTask) Clean() {
	close(mt.stopSignal)
}

// Move is used to create a move task to move paths to destination.
func Move(errCtrl ErrCtrl, dst string, paths ...string) error {
	return MoveWithContext(context.Background(), errCtrl, dst, paths...)
}

// MoveWithContext is used to create a move task with context to move paths to destination.
func MoveWithContext(ctx context.Context, errCtrl ErrCtrl, dst string, paths ...string) error {
	mt := NewMoveTask(errCtrl, nil, dst, paths...)
	return startTask(ctx, mt, "Move")
}
