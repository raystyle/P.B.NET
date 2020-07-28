package filemgr

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/looplab/fsm"
	"github.com/pkg/errors"

	"project/internal/module/task"
	"project/internal/system"
)

// zipTask implement task.Interface that is used to compress files to one zip file.
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

// Prepare is used to check destination is not exist or a directory.
func (ut *zipTask) Prepare(context.Context) error {
	return nil
}

func (ut *zipTask) Process(ctx context.Context, task *task.Task) error {
	return nil
}

func (ut *zipTask) Progress() string {
	return ""
}

func (ut *zipTask) Detail() string {
	ut.rwm.RLock()
	defer ut.rwm.RUnlock()
	return ut.detail
}

// Clean is used to send stop signal to watcher.
func (ut *zipTask) Clean() {
	close(ut.stopSignal)
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
