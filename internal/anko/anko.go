package anko

import (
	"context"
	"errors"
	"fmt"
	"os"

	"project/external/anko/ast"
	"project/external/anko/env"
	"project/external/anko/parser"
	"project/external/anko/vm"

	"project/internal/security"
)

// shortcut for env.Package.
var (
	Packages = env.Packages
	Types    = env.PackageTypes
)

// NewEnv is used to create a new global scope with packages.
func NewEnv() *Env {
	e := newEnv(env.NewEnv(), os.Stdout)
	e.ctx, e.cancel = context.WithCancel(context.Background())
	return e
}

// ParseSrc provides way to parse the code from source.
// Warning! source code will be covered after parse.
func ParseSrc(src string) (ast.Stmt, error) {
	defer security.CoverString(src)
	r := []rune(src)
	if len(r) < 1 {
		return nil, errors.New("empty source code")
	}
	defer security.CoverRunes(r)
	// prevent invalid code that will crash program
	// reference:
	// https://github.com/mattn/anko/issues/341
	if r[0] == '\ue031' {
		return nil, errors.New("invalid source code")
	}
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
	val, err := vm.RunContext(ctx, env.env, nil, stmt)
	if err != nil {
		if e, ok := err.(*vm.Error); ok {
			const format = "run with %s at line:%d column:%d"
			return val, fmt.Errorf(format, e.Message, e.Pos.Line, e.Pos.Column)
		}
		return val, err
	}
	return val, nil
}
