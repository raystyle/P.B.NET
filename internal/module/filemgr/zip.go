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
	dst      string // zip file path
	files    []string
	filesLen int

	skipDirs []string // store skipped directories

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
// If files is nil, it will create a zip file with empty files.
func NewZipTask(errCtrl ErrCtrl, callbacks fsm.Callbacks, dst string, files ...string) *task.Task {
	zt := zipTask{
		errCtrl:    errCtrl,
		dst:        dst,
		files:      files,
		filesLen:   len(files),
		current:    big.NewFloat(0),
		total:      big.NewFloat(0),
		stopSignal: make(chan struct{}),
	}
	return task.New(TaskNameZip, &zt, callbacks)
}

// Prepare is used to check destination file path is not exist.
func (zt *zipTask) Prepare(context.Context) error {
	dstAbs, err := filepath.Abs(zt.dst)
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
	zt.dst = dstAbs
	go zt.watcher()
	return nil
}

func (zt *zipTask) Process(ctx context.Context, task *task.Task) error {
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
// collect file info:
//   collect file information
//   path: testdata/test.dat
//
// compress file:
//   compress file, name: test.dat
//   src: C:\testdata\test.dat
//   dst: testdata/test.dat
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

// watcher is used to calculate current copy speed.
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
func Zip(errCtrl ErrCtrl, dst string, files ...string) error {
	return ZipWithContext(context.Background(), errCtrl, dst, files...)
}

// ZipWithContext is used to create a zip task with context to compress files into a zip file.
func ZipWithContext(ctx context.Context, errCtrl ErrCtrl, dst string, files ...string) error {
	zt := NewZipTask(errCtrl, nil, dst, files...)
	return startTask(ctx, zt, "Zip")
}

type dirToZipStat struct {
	srcAbs string
	dstAbs string
}

func zipCheckSrcDst(src, dst string) (string, string, bool, error) {
	// check the last is "*"
	var wildcard bool
	if l := len(src); l != 0 && src[l-1] == '*' {
		wildcard = true
		src = src[:l-1]
	}
	// get abs path
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return "", "", false, err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return "", "", false, err
	}
	// check src
	if wildcard {
		stat, err := os.Stat(srcAbs)
		if err != nil {
			return "", "", false, err
		}
		if !stat.IsDir() {
			return "", "", false, errors.New("use wildcard but src is a file")
		}
	}
	return srcAbs, dstAbs, wildcard, nil

}

// DirToZipFile is used to compress file or directory to a zip file and write.
// if the src with "*" at the end, it will not create a root directory to the zip file.
func DirToZipFile(src, dstZip string) error {

	// create zip file
	zipFile, err := system.OpenFile(src, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = zipFile.Close() }()
	// if compress failed, delete the zip file.
	var ok bool
	defer func() {
		if !ok {
			_ = zipFile.Close()
			_ = os.Remove(dstZip)
		}
	}()
	writer := zip.NewWriter(zipFile)
	// create root directory
	// if !wildcard {
	//
	// }

	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fmt.Println(path)

		header := zip.FileHeader{
			Name:     "",
			Method:   zip.Deflate,
			Modified: time.Time{},
		}
		_, _ = writer.CreateHeader(&header)

		return nil
	})
	if err != nil {
		return err
	}
	err = writer.Close()
	if err != nil {
		return err
	}
	ok = true
	return nil
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
