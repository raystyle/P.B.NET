package messages

import (
	"project/internal/guid"
)

// ExecuteShellCode is used to execute shellcode.
type ExecuteShellCode struct {
	Method    string
	ShellCode []byte
}

// SingleShell is used to run one command.
type SingleShell struct {
	ID      guid.GUID
	Command string
}

// SetID is used to set message id.
func (s *SingleShell) SetID(id *guid.GUID) {
	s.ID = *id
}

// SingleShellOutput is the output about run one command.
type SingleShellOutput struct {
	ID     guid.GUID
	Output []byte
	Err    string
}
