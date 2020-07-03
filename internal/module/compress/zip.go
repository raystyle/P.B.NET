package compress

import (
	"archive/zip"
	"fmt"
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

// ZipFileToDir is used to decompress zip file to a path.
// If appear the same file name, it will return a error.
func ZipFileToDir(src, dst string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return errors.Wrap(err, "failed to open zip file")
	}

	for _, file := range reader.File {
		err = zipWriteFile(file, dst)
		if err != nil {
			return err
		}
	}
	return nil
}

func zipWriteFile(file *zip.File, dst string) error {
	fileName := filepath.Join(dst, file.Name)
	// check file is already exists
	exist, err := system.IsExist(fileName)
	if err != nil {
		return errors.Wrap(err, "failed to check file path")
	}
	if exist {
		return errors.Errorf("file \"%s\" already exists", fileName)
	}
	mode := file.Mode()
	perm := mode.Perm()
	switch {
	case mode.IsRegular(): // write file
		// create file
		osFile, err := system.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
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
		err = os.MkdirAll(fileName, perm)
		if err != nil {
			return errors.Wrap(err, "failed to create directory")
		}
	default:
		return errors.Errorf("file \"%s\" with unknown mode: %s", file.Name, mode.String())
	}

	// set the modification time
	fmt.Println(file.Modified.String())
	n, offset := file.Modified.Zone()
	fmt.Println("zone:", n, offset)

	err = os.Chtimes(fileName, time.Now(), file.Modified)
	if err != nil {
		return errors.Wrap(err, "failed to change modification time")
	}
	return nil
}
