package taskmgr

import (
	"encoding/binary"
	"time"
	"unsafe"
)

// TaskList is used to get current process list.
type TaskList interface {
	GetProcesses() ([]*Process, error)
	Close()
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
	Username  string

	// for calculate CPU usage
	UserModeTime   uint64
	KernelModeTime uint64

	// for calculate Memory usage
	MemoryUsed uint64

	HandleCount uint32
	ThreadCount uint32

	IOReadBytes  uint64
	IOWriteBytes uint64

	Architecture   string
	CommandLine    string
	ExecutablePath string
	CreationDate   time.Time
}

// ID is used to identified this Process.
func (p *Process) ID() string {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(p.PID))
	return *(*string)(unsafe.Pointer(&b)) // #nosec
}

// for compare package
type processes []*Process

func (ps processes) Len() int {
	return len(ps)
}

func (ps processes) ID(i int) string {
	return ps[i].ID()
}
