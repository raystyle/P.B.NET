package system

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// OpenFile is used to open file, if directory is not exists, it will create it.
func OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	dir, _ := filepath.Split(name)
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

// GetConnHandle is used to get handle about raw connection.
func GetConnHandle(conn syscall.Conn) (uintptr, error) {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return 0, err
	}
	var f uintptr
	err = rawConn.Control(func(fd uintptr) {
		f = fd
	})
	if err != nil {
		return 0, err
	}
	return f, nil
}

// ExecutableName is used to get the executable file name.
func ExecutableName() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	_, file := filepath.Split(path)
	return file, nil
}

// ChangeCurrentDirectory is used to changed path for service program and prevent
// to get invalid path when running test.
func ChangeCurrentDirectory() error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	dir, _ := filepath.Split(path)
	return os.Chdir(dir)
}

// CheckError is used to check error is nil, if err is not nil, it will print error
// and exit program with code 1.
func CheckError(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
