// +build windows

package taskmgr

type taskList struct {
}

func (tl *taskList) GetProcesses() ([]*Process, error) {
	return nil, nil
}

func newTaskList() (*taskList, error) {
	return nil, nil
}
