package vm

import (
	"context"
	"fmt"
	"reflect"

	"project/external/anko/ast"
	"project/external/anko/env"
	"project/external/anko/parser"
)

// Options provides options to run VM with.
type Options struct {
	Debug bool // run in Debug mode
}

var (
	stringType         = reflect.TypeOf("a")
	byteType           = reflect.TypeOf(byte('a'))
	runeType           = reflect.TypeOf('a')
	interfaceType      = reflect.ValueOf([]interface{}{int64(1)}).Index(0).Type()
	interfaceSliceType = reflect.TypeOf([]interface{}{})
	reflectValueType   = reflect.TypeOf(reflect.Value{})
	errorType          = reflect.ValueOf([]error{nil}).Index(0).Type()
	vmErrorType        = reflect.TypeOf(&Error{})
	contextType        = reflect.TypeOf((*context.Context)(nil)).Elem()

	nilValue                  = reflect.New(reflect.TypeOf((*interface{})(nil)).Elem()).Elem()
	trueValue                 = reflect.ValueOf(true)
	falseValue                = reflect.ValueOf(false)
	reflectValueNilValue      = reflect.ValueOf(nilValue)
	reflectValueErrorNilValue = reflect.ValueOf(reflect.New(errorType).Elem())
)

// Execute parses script and executes in the specified environment.
func Execute(env *env.Env, opts *Options, script string) (interface{}, error) {
	stmt, err := parser.ParseSrc(script)
	if err != nil {
		return nilValue, err
	}
	return RunContext(context.Background(), env, opts, stmt)
}

// ExecuteContext parses script and executes in the specified environment with context.
func ExecuteContext(ctx context.Context, env *env.Env, opts *Options, script string) (interface{}, error) {
	stmt, err := parser.ParseSrc(script)
	if err != nil {
		return nilValue, err
	}
	return RunContext(ctx, env, opts, stmt)
}

// Run executes statement in the specified environment.
func Run(env *env.Env, options *Options, stmt ast.Stmt) (interface{}, error) {
	return RunContext(context.Background(), env, options, stmt)
}

// RunContext executes statement in the specified environment with context.
func RunContext(ctx context.Context, env *env.Env, opts *Options, stmt ast.Stmt) (interface{}, error) {
	runInfo := runInfoStruct{ctx: ctx, env: env, options: opts, stmt: stmt, rv: nilValue}
	if runInfo.options == nil {
		runInfo.options = &Options{}
	}
	runInfo.runSingleStmt()
	if runInfo.err == ErrReturn {
		runInfo.err = nil
	}
	return runInfo.rv.Interface(), runInfo.err
}

// recoverFunc generic recover function.
func recoverFunc(runInfo *runInfoStruct) {
	recoverInterface := recover()
	if recoverInterface == nil {
		return
	}
	switch value := recoverInterface.(type) {
	case *Error:
		runInfo.err = value
	case error:
		runInfo.err = value
	default:
		runInfo.err = fmt.Errorf("%v", recoverInterface)
	}
}

func makeType(runInfo *runInfoStruct, typeStruct *ast.TypeStruct) reflect.Type {
	switch typeStruct.Kind {
	case ast.TypeDefault:
		return getTypeFromEnv(runInfo, typeStruct)
	case ast.TypePtr:
		var t reflect.Type
		if typeStruct.SubType != nil {
			t = makeType(runInfo, typeStruct.SubType)
		} else {
			t = getTypeFromEnv(runInfo, typeStruct)
		}
		if runInfo.err != nil {
			return nil
		}
		if t == nil {
			return nil
		}
		return reflect.PtrTo(t)
	case ast.TypeSlice:
		var t reflect.Type
		if typeStruct.SubType != nil {
			t = makeType(runInfo, typeStruct.SubType)
		} else {
			t = getTypeFromEnv(runInfo, typeStruct)
		}
		if runInfo.err != nil {
			return nil
		}
		if t == nil {
			return nil
		}
		for i := 1; i < typeStruct.Dimensions; i++ {
			t = reflect.SliceOf(t)
		}
		return reflect.SliceOf(t)
	case ast.TypeMap:
		key := makeType(runInfo, typeStruct.Key)
		if runInfo.err != nil {
			return nil
		}
		if key == nil {
			return nil
		}
		t := makeType(runInfo, typeStruct.SubType)
		if runInfo.err != nil {
			return nil
		}
		if t == nil {
			return nil
		}
		if !runInfo.options.Debug {
			// captures panic
			defer recoverFunc(runInfo)
		}
		t = reflect.MapOf(key, t)
		return t
	case ast.TypeChan:
		var t reflect.Type
		if typeStruct.SubType != nil {
			t = makeType(runInfo, typeStruct.SubType)
		} else {
			t = getTypeFromEnv(runInfo, typeStruct)
		}
		if runInfo.err != nil {
			return nil
		}
		if t == nil {
			return nil
		}
		return reflect.ChanOf(reflect.BothDir, t)
	case ast.TypeStructType:
		var t reflect.Type
		fields := make([]reflect.StructField, 0, len(typeStruct.StructNames))
		for i := 0; i < len(typeStruct.StructNames); i++ {
			t = makeType(runInfo, typeStruct.StructTypes[i])
			if runInfo.err != nil {
				return nil
			}
			if t == nil {
				return nil
			}
			fields = append(fields, reflect.StructField{Name: typeStruct.StructNames[i], Type: t})
		}
		if !runInfo.options.Debug {
			// captures panic
			defer recoverFunc(runInfo)
		}
		t = reflect.StructOf(fields)
		return t
	default:
		runInfo.err = fmt.Errorf("unknown kind")
		return nil
	}
}

func makeValue(t reflect.Type) (reflect.Value, error) {
	switch t.Kind() {
	case reflect.Chan:
		return reflect.MakeChan(t, 0), nil
	case reflect.Func:
		return reflect.MakeFunc(t, nil), nil
	case reflect.Map:
		// note creating slice as work around to create map
		// just doing MakeMap can give incorrect type for defined types
		value := reflect.MakeSlice(reflect.SliceOf(t), 0, 1)
		value = reflect.Append(value, reflect.MakeMap(reflect.MapOf(t.Key(), t.Elem())))
		return value.Index(0), nil
	case reflect.Ptr:
		ptrV := reflect.New(t.Elem())
		v, err := makeValue(t.Elem())
		if err != nil {
			return nilValue, err
		}
		ptrV.Elem().Set(v)
		return ptrV, nil
	case reflect.Slice:
		return reflect.MakeSlice(t, 0, 0), nil
	}
	return reflect.New(t).Elem(), nil
}
