package vm

import (
	"project/external/anko/ast"
)

// Error is a VM run error.
type Error struct {
	Message string
	Pos     ast.Position
}

// Error returns the VM error message.
func (e *Error) Error() string {
	return e.Message
}

// newError makes VM error from error.
func newError(pos ast.Pos, err error) error {
	if err == nil {
		return nil
	}
	if pos == nil {
		return &Error{Message: err.Error(), Pos: ast.Position{Line: 1, Column: 1}}
	}
	return &Error{Message: err.Error(), Pos: pos.Position()}
}

// newStringError makes VM error from string.
func newStringError(pos ast.Pos, err string) error {
	if err == "" {
		return nil
	}
	if pos == nil {
		return &Error{Message: err, Pos: ast.Position{Line: 1, Column: 1}}
	}
	return &Error{Message: err, Pos: pos.Position()}
}
