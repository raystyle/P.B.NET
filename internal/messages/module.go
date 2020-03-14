package messages

import (
	"project/internal/guid"
)

// ShellCode is used to execute shellcode.
type ShellCode struct {
	ID        guid.GUID
	Method    string
	ShellCode []byte
}

// SetID is used to set message id.
func (e *ShellCode) SetID(id *guid.GUID) {
	e.ID = *id
}

// ShellCodeResult is the result about execute shellcode.
type ShellCodeResult struct {
	ID  guid.GUID
	Err string
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
