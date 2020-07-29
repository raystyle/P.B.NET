package filemgr

import (
	"archive/zip"
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
	"project/internal/system"
	"project/internal/xpanic"
)

// zipTask implement task.Interface that is used to compress files into a zip file.
// It can pause in progress and get current progress and detail information.
type zipTask struct {
	errCtrl  ErrCtrl
	zipPath  string   // zip file absolute path that be created
	paths    []string // absolute path that will be compressed
	pathsLen int

	basePath  string      // for filepath.Rel() in Process
	files     []*fileStat // store all files stats that will be compressed
	skipDirs  []string    // store skipped directories
	zipWriter *zip.Writer

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

// NewZipTask is used to create a zip task that implement task.Interface.
// If files is nil, it will create a zip file with empty file.
// Files must in the same directory.
func NewZipTask(errCtrl ErrCtrl, callbacks fsm.Callbacks, zipPath string, paths ...string) *task.Task {
	zt := zipTask{
		errCtrl:    errCtrl,
		zipPath:    zipPath,
		paths:      paths,
		pathsLen:   len(paths),
		current:    big.NewFloat(0),
		total:      big.NewFloat(0),
		stopSignal: make(chan struct{}),
	}
	return task.New(TaskNameZip, &zt, callbacks)
}

// Prepare is used to check destination file path is not exist.
func (zt *zipTask) Prepare(context.Context) error {
	// check destination path
	dstAbs, err := filepath.Abs(zt.zipPath)
	if err != nil {
		return errors.Wrap(err, "failed to get absolute file path")
	}
	dstStat, err := stat(dstAbs)
	if err != nil {
		return err
	}
	if dstStat != nil {
		return errors.Errorf("destination path %s is already exists", dstAbs)
	}
	zt.zipPath = dstAbs
	// check files
	if zt.pathsLen == 0 {
		return errors.New("empty path")
	}
	// check path is valid
	paths := make(map[string]struct{}, zt.pathsLen)
	for i := 0; i < zt.pathsLen; i++ {
		if zt.paths[i] == "" {
			return errors.New("appear empty path in source path")
		}
		// make sure all source path is absolute
		absPath, err := filepath.Abs(zt.paths[i])
		if err != nil {
			return errors.Wrap(err, "failed to get absolute file path")
		}
		zt.paths[i] = absPath
		if i == 0 {
			paths[absPath] = struct{}{}
			zt.basePath = filepath.Dir(absPath)
			continue
		}
		// only exist one root path
		if isRoot(absPath) {
			return errors.Errorf("appear root path \"%s\"", absPath)
		}
		// check file path is already exists
		_, ok := paths[absPath]
		if ok {
			return errors.Errorf("appear the same path \"%s\"", absPath)
		}
		// compare directory is same as the first path
		dir := filepath.Dir(absPath)
		if dir != zt.basePath {
			const format = "split directory about source \"%s\" is different with \"%s\""
			return errors.Errorf(format, absPath, zt.paths[0])
		}
		paths[absPath] = struct{}{}
	}
	zt.files = make([]*fileStat, 0, 64)
	go zt.watcher()
	return nil
}

func (zt *zipTask) Process(ctx context.Context, task *task.Task) error {
	defer zt.updateDetail("finished")
	// must collect files information because the zip file in the same path
	for i := 0; i < zt.pathsLen; i++ {
		err := zt.collectPathInfo(ctx, task, zt.paths[i])
		if err != nil {
			return err
		}
	}
	// create zip file
	zipFile, err := system.OpenFile(zt.zipPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.Wrap(err, "failed to create zip file")
	}
	defer func() { _ = zipFile.Close() }()
	zt.zipWriter = zip.NewWriter(zipFile)
	// check files is empty
	l := len(zt.files)
	if l == 0 {
		// set fake progress for pass progress check
		zt.rwm.Lock()
		defer zt.rwm.Unlock()
		zt.current.SetUint64(1)
		zt.total.SetUint64(1)
		return nil
	}
	// compress files and add directories
	for i := 0; i < l; i++ {
		err := zt.compress(ctx, task, zt.files[i])
		if err != nil {
			return err
		}
	}
	return zipFile.Sync()
}

func (zt *zipTask) collectPathInfo(ctx context.Context, task *task.Task, srcPath string) error {
	walkFunc := func(path string, stat os.FileInfo, err error) error {
		if err != nil {
			ps := noticePs{
				ctx:     ctx,
				task:    task,
				errCtrl: zt.errCtrl,
			}
			const format = "failed to walk \"%s\" in \"%s\": %s"
			err = fmt.Errorf(format, path, srcPath, err)
			skip, ne := noticeFailedToCollect(&ps, path, err)
			if skip {
				return filepath.SkipDir
			}
			return ne
		}
		// check task is canceled
		if task.Canceled() {
			return context.Canceled
		}
		zt.files = append(zt.files, &fileStat{
			path: path,
			stat: stat,
		})
		// update detail and total size
		if stat.IsDir() {
			// collect directory information
			// path: C:\testdata\test
			const format = "collect directory information\npath: %s"
			zt.updateDetail(fmt.Sprintf(format, path))
			return nil
		}
		// collect file information
		// path: C:\testdata\test.dat
		const format = "collect file information\npath: %s"
		zt.updateDetail(fmt.Sprintf(format, path))
		zt.updateTotal(stat.Size(), true)
		return nil
	}
	return filepath.Walk(srcPath, walkFunc)
}

func (zt *zipTask) compress(ctx context.Context, task *task.Task, file *fileStat) error {
	// skip file if it in skipped directories
	for i := 0; i < len(zt.skipDirs); i++ {
		if strings.HasPrefix(file.path, zt.skipDirs[i]) {
			zt.updateCurrent(file.stat.Size(), true)
			return nil
		}
	}
	// not recovered
	relPath, err := filepath.Rel(zt.basePath, file.path)
	if err != nil {
		return err
	}
	// file is root directory
	if relPath == "." {
		return nil
	}
	if file.stat.IsDir() {
		return zt.mkdir(ctx, task, file.path, relPath)
	}

	return nil
}

func (zt *zipTask) mkdir(ctx context.Context, task *task.Task, dirPath, relPath string) error {
	// update current task detail, output:
	//   create directory, name: testdata
	//   src: C:\testdata
	//   dst: zip/testdata
	const format = "create directory, name: %s\nsrc: %s\ndst: zip/%s"
	dirName := filepath.Base(dirPath)
	zt.updateDetail(fmt.Sprintf(format, dirName, dirPath, relPath))
	// update modification time
retry:
	// check task is canceled
	if task.Canceled() {
		return context.Canceled
	}
	dirStat, err := os.Stat(dirPath)
	if err != nil {
		ps := noticePs{
			ctx:     ctx,
			task:    task,
			errCtrl: zt.errCtrl,
		}
		retry, ne := noticeFailedToZip(&ps, dirPath, err)
		if retry {
			goto retry
		}
		if ne != nil {
			return ne
		}
		zt.skipDirs = append(zt.skipDirs, dirPath)
		return nil
	}
	// create a directory
	header := zip.FileHeader{
		Name:     relPath + "/",
		Method:   zip.Store,
		Modified: dirStat.ModTime(),
	}
	_, err = zt.zipWriter.CreateHeader(&header)
	if err != nil {
		ps := noticePs{
			ctx:     ctx,
			task:    task,
			errCtrl: zt.errCtrl,
		}
		retry, ne := noticeFailedToZip(&ps, dirPath, err)
		if retry {
			goto retry
		}
		if ne != nil {
			return ne
		}
		zt.skipDirs = append(zt.skipDirs, dirPath)
	}
	return nil
}

// Progress is used to get progress about current zip task.
//
// collect: "0%"
// zip:     "15.22%|current/total|128 MB/s"
// finish:  "100%"
func (zt *zipTask) Progress() string {
	zt.rwm.RLock()
	defer zt.rwm.RUnlock()
	// prevent / 0
	if zt.total.Cmp(zeroFloat) == 0 {
		return "0%"
	}
	switch zt.current.Cmp(zt.total) {
	case 0: // current == total
		return "100%"
	case 1: // current > total
		current := zt.current.Text('G', 64)
		total := zt.total.Text('G', 64)
		return fmt.Sprintf("err: current %s > total %s", current, total)
	}
	value := new(big.Float).Quo(zt.current, zt.total)
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
	current := zt.current.Text('G', 64)
	total := zt.total.Text('G', 64)
	speed := convert.FormatByte(zt.speed)
	return fmt.Sprintf("%s%%|%s/%s|%s/s", progress, current, total, speed)
}

func (zt *zipTask) updateCurrent(delta int64, add bool) {
	zt.rwm.Lock()
	defer zt.rwm.Unlock()
	d := new(big.Float).SetInt64(delta)
	if add {
		zt.current.Add(zt.current, d)
	} else {
		zt.current.Sub(zt.current, d)
	}
}

func (zt *zipTask) updateTotal(delta int64, add bool) {
	zt.rwm.Lock()
	defer zt.rwm.Unlock()
	d := new(big.Float).SetInt64(delta)
	if add {
		zt.total.Add(zt.total, d)
	} else {
		zt.total.Sub(zt.total, d)
	}
}

// Detail is used to get detail about zip task.
// collect file or directory info:
//   collect file information
//   path: C:\testdata\test.dat
//
// compress file:
//   compress file, name: test.dat
//   src: C:\testdata\test.dat
//   dst: zip/testdata/test.dat
func (zt *zipTask) Detail() string {
	zt.rwm.RLock()
	defer zt.rwm.RUnlock()
	return zt.detail
}

func (zt *zipTask) updateDetail(detail string) {
	zt.rwm.Lock()
	defer zt.rwm.Unlock()
	zt.detail = detail
}

// watcher is used to calculate current compress speed.
func (zt *zipTask) watcher() {
	defer func() {
		if r := recover(); r != nil {
			xpanic.Log(r, "zipTask.watcher")
		}
	}()
	ticker := time.NewTicker(time.Second / time.Duration(len(zt.speeds)))
	defer ticker.Stop()
	current := new(big.Float)
	index := -1
	for {
		select {
		case <-ticker.C:
			index++
			if index >= len(zt.speeds) {
				index = 0
			}
			zt.watchSpeed(current, index)
		case <-zt.stopSignal:
			return
		}
	}
}

func (zt *zipTask) watchSpeed(current *big.Float, index int) {
	zt.rwm.Lock()
	defer zt.rwm.Unlock()
	delta := new(big.Float).Sub(zt.current, current)
	current.Add(current, delta)
	// update speed
	zt.speeds[index], _ = delta.Uint64()
	if zt.full {
		zt.speed = 0
		for i := 0; i < len(zt.speeds); i++ {
			zt.speed += zt.speeds[i]
		}
		return
	}
	if index == len(zt.speeds)-1 {
		zt.full = true
	}
	// calculate average speed
	var speed float64 // current speed
	for i := 0; i < index+1; i++ {
		speed += float64(zt.speeds[i])
	}
	zt.speed = uint64(speed / float64(index+1) * float64(len(zt.speeds)))
}

// Clean is used to send stop signal to watcher.
func (zt *zipTask) Clean() {
	close(zt.stopSignal)
}

// Zip is used to create a zip task to compress files into a zip file.
func Zip(errCtrl ErrCtrl, zipPath string, paths ...string) error {
	return ZipWithContext(context.Background(), errCtrl, zipPath, paths...)
}

// ZipWithContext is used to create a zip task with context to compress files into a zip file.
func ZipWithContext(ctx context.Context, errCtrl ErrCtrl, zipPath string, paths ...string) error {
	zt := NewZipTask(errCtrl, nil, zipPath, paths...)
	return startTask(ctx, zt, "Zip")
}

// ZipFileToDir is used to decompress zip file to a directory.
// If appear the same file name, it will return a error.
func ZipFileToDir(srcZip, dstDir string) error {
	reader, err := zip.OpenReader(srcZip)
	if err != nil {
		return errors.Wrap(err, "failed to open zip file")
	}
	dirs := make([]*zip.File, 0, len(reader.File)/5)
	for _, file := range reader.File {
		err = zipWriteFile(file, dstDir)
		if err != nil {
			return err
		}
		if file.Mode().IsDir() {
			dirs = append(dirs, file)
		}
	}
	for _, dir := range dirs {
		filename := filepath.Join(dstDir, filepath.Clean(dir.Name))
		err = os.Chtimes(filename, time.Now(), dir.Modified)
		if err != nil {
			return errors.Wrap(err, "failed to change directory \"\" modification time")
		}
	}
	return nil
}

func zipWriteFile(file *zip.File, dst string) error {
	filename := filepath.Join(dst, filepath.Clean(file.Name))
	// check file is already exists
	exist, err := system.IsExist(filename)
	if err != nil {
		return errors.Wrap(err, "failed to check file path")
	}
	if exist {
		return errors.Errorf("file \"%s\" already exists", filename)
	}
	mode := file.Mode()
	perm := mode.Perm()
	switch {
	case mode.IsRegular(): // write file
		// create file
		osFile, err := system.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return errors.Wrap(err, "failed to create file")
		}
		defer func() { _ = osFile.Close() }()
		// write data
		rc, err := file.Open()
		if err != nil {
			return errors.Wrapf(err, "failed to open file \"%s\" in zip file", file.Name)
		}
		defer func() { _ = rc.Close() }()
		_, err = io.Copy(osFile, rc)
		if err != nil {
			return errors.Wrap(err, "failed to copy file")
		}
	case mode.IsDir(): // create directory
		err = os.MkdirAll(filename, perm)
		if err != nil {
			return errors.Wrap(err, "failed to create directory")
		}
	default: // skip unknown mode
		// add logger
		return nil
	}
	err = os.Chtimes(filename, time.Now(), file.Modified)
	if err != nil {
		return errors.Wrap(err, "failed to change modification time")
	}
	return nil
}
