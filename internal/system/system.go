package system

import (
	"os"
	"path/filepath"
	"syscall"

	"project/internal/logger"
)

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

// ChangeCurrentDirectory is used to changed path for service program
// and prevent get invalid path when test.
func ChangeCurrentDirectory() error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	dir, _ := filepath.Split(path)
	return os.Chdir(dir)
}

// SetErrorLogger is used to log error before service program start.
// If occur some error before start, you can get it.
func SetErrorLogger(name string) (*os.File, error) {
	file, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND, 0600) // #nosec
	if err != nil {
		return nil, err
	}
	mLogger := logger.NewMultiLogger(logger.Error, os.Stdout, file)
	logger.HijackLogWriter(logger.Error, "init", mLogger, 0)
	return file, nil
}
