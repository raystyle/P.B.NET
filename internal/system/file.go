package system

import (
	"os"
	"path/filepath"
)

// OpenFile is used to open file, if directory is not exists, it will create it.
func OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	dir := filepath.Dir(name)
	if dir != "" {
		err := os.MkdirAll(dir, 0750)
		if err != nil {
			return nil, err
		}
	}
	return os.OpenFile(name, flag, perm) // #nosec
}

// WriteFile is used to write file and call synchronize, it used to write small file.
func WriteFile(filename string, data []byte) error {
	file, err := OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if e := file.Sync(); err == nil {
		err = e
	}
	if e := file.Close(); err == nil {
		err = e
	}
	return err
}

// IsExist is used to check the target path or file is exist.
func IsExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// IsNotExist is used to check the target path or file is not exist.
func IsNotExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return false, nil
	}
	if os.IsNotExist(err) {
		return true, nil
	}
	return false, err
}
