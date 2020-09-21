package vm

import (
	"context"
	"errors"
	"reflect"

	"project/external/anko/ast"
	"project/external/anko/env"
)

var (
	// ErrBreak when there is an unexpected break statement
	ErrBreak = errors.New("unexpected break statement")
	// ErrContinue when there is an unexpected continue statement
	ErrContinue = errors.New("unexpected continue statement")
	// ErrReturn when there is an unexpected return statement
	ErrReturn = errors.New("unexpected return statement")
	// ErrInterrupt when execution has been interrupted
	ErrInterrupt = errors.New("execution interrupted")
)

// runInfo provides run incoming and outgoing information.
type runInfoStruct struct {
	// incoming
	ctx      context.Context
	env      *env.Env
	options  *Options
	stmt     ast.Stmt
	expr     ast.Expr
	operator ast.Operator

	// outgoing
	rv  reflect.Value
	err error
}

// runSingleStmt executes statement in the specified environment with context.
func (runInfo *runInfoStruct) runSingleStmt() {

}
