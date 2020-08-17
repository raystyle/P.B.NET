package taskmgr

// TaskList is used to get current process list.
type TaskList interface {
	GetProcesses() ([]*Process, error)
}

// NewTaskList is used to create a new TaskList tool.
func NewTaskList() (TaskList, error) {
	return newTaskList()
}

// Process contains information about process.
type Process struct {
	Name string
	PID  int64
	PPID int64

	SessionID uint32
	UserName  string // TODO finish it

	// for calculate CPU usage
	UserModeTime   uint64
	KernelModeTime uint64

	MemoryUsed uint64

	HandleCount uint32
	ThreadCount uint32

	IOWriteBytes uint64
	IOReadBytes  uint64

	CommandLine    string
	ExecutablePath string
}
