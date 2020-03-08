package messages

// ExecuteShellCode is used to execute shellcode.
type ExecuteShellCode struct {
	Method    string
	ShellCode []byte
}

// SingleShell is used to run one command.
type SingleShell struct {
	ID      uint64
	Command string
}

// SetID is used to set message id.
func (s *SingleShell) SetID(id uint64) {
	s.ID = id
}

// SingleShellOutput is the output about run one command.
type SingleShellOutput struct {
	ID     uint64
	Output []byte
	Err    string
}
