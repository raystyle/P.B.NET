package anko

import (
	"context"
	"fmt"
	"io"

	"github.com/mattn/anko/ast"
	"github.com/mattn/anko/env"
	"github.com/mattn/anko/parser"
	"github.com/mattn/anko/vm"

	"project/internal/security"
)

// shortcut for env.Package.
var (
	Packages = env.Packages
	Types    = env.PackageTypes
)

// NewEnv is used to create an new global scope with packages.
func NewEnv(output io.Writer) *Env {
	return newEnv(output)
}

// ParseSrc provides way to parse the code from source.
func ParseSrc(src string) (ast.Stmt, error) {
	defer security.CoverString(src)
	stmt, err := parser.ParseSrc(src)
	if err != nil {
		const format = "parse source with %s at line:%d column:%d"
		e := err.(*parser.Error)
		return nil, fmt.Errorf(format, e.Message, e.Pos.Line, e.Pos.Column)
	}
	return stmt, nil
}

// Run executes statement in the specified environment.
func Run(env *Env, stmt ast.Stmt) (interface{}, error) {
	return RunContext(context.Background(), env, stmt)
}

// RunContext executes statement in the specified environment with context.
func RunContext(ctx context.Context, env *Env, stmt ast.Stmt) (interface{}, error) {
	val, err := vm.RunContext(ctx, env.Env, nil, stmt)
	if err != nil {
		if e, ok := err.(*vm.Error); ok {
			const format = "run with %s at line:%d column:%d"
			return val, fmt.Errorf(format, e.Message, e.Pos.Line, e.Pos.Column)
		}
		return val, err
	}
	return val, nil
}
