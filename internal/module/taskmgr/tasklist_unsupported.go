// +build !windows

package taskmgr

import (
	"errors"
	"runtime"
)

func newTaskList() (TaskList, error) {
	return nil, errors.New("tasklist is not implemented on " + runtime.GOOS)
}
