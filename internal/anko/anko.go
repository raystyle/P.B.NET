package anko

import (
	"context"
	"fmt"
	"reflect"

	"github.com/mattn/anko/ast"
	"github.com/mattn/anko/core"
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
func NewEnv() *env.Env {
	e := env.NewEnv()
	core.ImportToX(e)
	defineBasicType(e)
	defineCoreFunc(e)
	return e
}

func defineBasicType(e *env.Env) {
	_ = e.DefineType("int8", int8(1))
	_ = e.DefineType("int16", int16(1))
	_ = e.DefineType("uint8", uint8(1))
	_ = e.DefineType("uint16", uint16(1))
	_ = e.DefineType("uintptr", uintptr(1))
}

// defineCoreFunc is used to add core function.
// core.Import() with leaks, so we implement it self.
func defineCoreFunc(e *env.Env) {
	_ = e.Define("keys", coreKeys)
	_ = e.Define("range", coreRange)

	_ = e.Define("print", fmt.Print)
	_ = e.Define("println", fmt.Println)
	_ = e.Define("printf", fmt.Printf)

	_ = e.Define("typeOf", func(v interface{}) string {
		return reflect.TypeOf(v).String()
	})

	_ = e.Define("kindOf", func(v interface{}) string {
		typeOf := reflect.TypeOf(v)
		if typeOf == nil {
			return "nil"
		}
		return typeOf.Kind().String()
	})

	childEnv := e.DeepCopy()
	_ = e.Define("eval", func(src string) interface{} {
		return coreEval(childEnv, src)
	})
}

func coreKeys(v interface{}) []interface{} {
	rv := reflect.ValueOf(v)
	mapKeysValue := rv.MapKeys()
	mapKeys := make([]interface{}, len(mapKeysValue))
	for i := 0; i < len(mapKeysValue); i++ {
		mapKeys[i] = mapKeysValue[i].Interface()
	}
	return mapKeys
}

func coreRange(args ...int64) []int64 {
	var start, stop int64
	var step int64 = 1

	switch len(args) {
	case 0:
		panic("range expected at least 1 argument, got 0")
	case 1:
		stop = args[0]
	case 2:
		start = args[0]
		stop = args[1]
	case 3:
		start = args[0]
		stop = args[1]
		step = args[2]
		if step == 0 {
			panic("range argument 3 must not be zero")
		}
	default:
		panic(fmt.Sprintf("range expected at most 3 arguments, got %d", len(args)))
	}

	var arr []int64
	for i := start; (step > 0 && i < stop) || (step < 0 && i > stop); i += step {
		arr = append(arr, i)
	}
	return arr
}

func coreEval(env *env.Env, src string) interface{} {
	stmt, err := ParseSrc(src)
	if err != nil {
		panic(err)
	}
	val, err := vm.Run(env.DeepCopy(), nil, stmt)
	if err != nil {
		panic(err)
	}
	return val
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
func Run(env *env.Env, stmt ast.Stmt) (interface{}, error) {
	return RunContext(context.Background(), env, stmt)
}

// RunContext executes statement in the specified environment with context.
func RunContext(ctx context.Context, env *env.Env, stmt ast.Stmt) (interface{}, error) {
	val, err := vm.RunContext(ctx, env, nil, stmt)
	if err != nil {
		if e, ok := err.(*vm.Error); ok {
			const format = "run with %s at line:%d column:%d"
			return val, fmt.Errorf(format, e.Message, e.Pos.Line, e.Pos.Column)
		}
		return val, err
	}
	return val, nil
}
