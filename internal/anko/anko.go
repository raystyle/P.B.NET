package anko

import (
	"context"
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/mattn/anko/ast"
	"github.com/mattn/anko/core"
	"github.com/mattn/anko/env"
	"github.com/mattn/anko/parser"
	"github.com/mattn/anko/vm"

	"project/internal/security"
)

// NewEnv is used to create an new global scope with packages.
func NewEnv() *env.Env {
	e := env.NewEnv()
	core.ImportToX(e)
	addCore(e)
	return e
}

// addCore is used to add core function.
// core.Import() with leaks, so we implement it self.
func addCore(e *env.Env) {
	_ = e.Define("keys", func(v interface{}) []interface{} {
		rv := reflect.ValueOf(v)
		mapKeysValue := rv.MapKeys()
		mapKeys := make([]interface{}, len(mapKeysValue))
		for i := 0; i < len(mapKeysValue); i++ {
			mapKeys[i] = mapKeysValue[i].Interface()
		}
		return mapKeys
	})

	_ = e.Define("range", func(args ...int64) []int64 {
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
	})

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
	_ = e.Define("eval", func(s string) interface{} {
		body, err := ioutil.ReadFile(s)
		if err != nil {
			panic(err)
		}
		scanner := new(parser.Scanner)
		scanner.Init(string(body))
		stmts, err := parser.Parse(scanner)
		if err != nil {
			if pe, ok := err.(*parser.Error); ok {
				pe.Filename = s
				panic(pe)
			}
			panic(err)
		}
		rv, err := vm.Run(childEnv.DeepCopy(), nil, stmts)
		if err != nil {
			panic(err)
		}
		return rv
	})
	_ = e.Define("print", fmt.Print)
	_ = e.Define("println", fmt.Println)
	_ = e.Define("printf", fmt.Printf)
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
