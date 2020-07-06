package filemgr

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"project/internal/system"
)

// DirToZipFile is used to compress file or directory to a zip file and write.
func DirToZipFile() {

}

// ZipFileToDir is used to decompress zip file to a directory.
// If appear the same file name, it will return a error.
func ZipFileToDir(src, dst string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return errors.Wrap(err, "failed to open zip file")
	}
	dirs := make([]*zip.File, 0, len(reader.File)/5)
	for _, file := range reader.File {
		err = zipWriteFile(file, dst)
		if err != nil {
			return err
		}
		if file.Mode().IsDir() {
			dirs = append(dirs, file)
		}
	}
	for _, dir := range dirs {
		filename := filepath.Join(dst, dir.Name)
		err = os.Chtimes(filename, time.Now(), dir.Modified)
		if err != nil {
			return errors.Wrap(err, "failed to change directory \"\" modification time")
		}
	}
	return nil
}

func zipWriteFile(file *zip.File, dst string) error {
	filename := filepath.Join(dst, file.Name)
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
