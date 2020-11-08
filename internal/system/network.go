package system

import (
	"syscall"
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
