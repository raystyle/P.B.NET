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

func isNil(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		// from reflect IsNil:
		// Note that IsNil is not always equivalent to a regular comparison with nil in Go.
		// For example, if v was created by calling ValueOf with an uninitialized interface variable i,
		// i==nil will be true but v.IsNil will panic as v will be the zero Value.
		return v.IsNil()
	default:
		return false
	}
}

func isNum(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func getMapIndex(key reflect.Value, aMap reflect.Value) reflect.Value {
	if aMap.IsNil() {
		return nilValue
	}

	var err error
	key, err = convertReflectValueToType(key, aMap.Type().Key())
	if err != nil {
		return nilValue
	}

	// From reflect MapIndex:
	// It returns the zero Value if key is not found in the map or if v represents a nil map.
	value := aMap.MapIndex(key)
	if !value.IsValid() {
		return nilValue
	}

	if aMap.Type().Elem() == interfaceType && !value.IsNil() {
		value = reflect.ValueOf(value.Interface())
	}

	return value
}

// appendSlice appends rhs to lhs, function assumes lhsV and rhsV are slice or array.
// nolint: gocyclo
//gocyclo:ignore
func appendSlice(expr ast.Expr, lhsV reflect.Value, rhsV reflect.Value) (reflect.Value, error) {
	lhsT := lhsV.Type().Elem()
	rhsT := rhsV.Type().Elem()

	if lhsT == rhsT {
		return reflect.AppendSlice(lhsV, rhsV), nil
	}

	if rhsT.ConvertibleTo(lhsT) {
		for i := 0; i < rhsV.Len(); i++ {
			lhsV = reflect.Append(lhsV, rhsV.Index(i).Convert(lhsT))
		}
		return lhsV, nil
	}

	leftHasSubArray := lhsT.Kind() == reflect.Slice || lhsT.Kind() == reflect.Array
	rightHasSubArray := rhsT.Kind() == reflect.Slice || rhsT.Kind() == reflect.Array

	if leftHasSubArray != rightHasSubArray && lhsT != interfaceType && rhsT != interfaceType {
		return nilValue, newStringError(expr, "invalid type conversion")
	}

	if !leftHasSubArray && !rightHasSubArray {
		for i := 0; i < rhsV.Len(); i++ {
			value := rhsV.Index(i)
			if rhsT == interfaceType {
				value = value.Elem()
			}
			if lhsT == value.Type() {
				lhsV = reflect.Append(lhsV, value)
			} else if value.Type().ConvertibleTo(lhsT) {
				lhsV = reflect.Append(lhsV, value.Convert(lhsT))
			} else {
				return nilValue, newStringError(expr, "invalid type conversion")
			}
		}
		return lhsV, nil
	}

	if (leftHasSubArray || lhsT == interfaceType) && (rightHasSubArray || rhsT == interfaceType) {
		for i := 0; i < rhsV.Len(); i++ {
			value := rhsV.Index(i)
			if rhsT == interfaceType {
				value = value.Elem()
				if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
					return nilValue, newStringError(expr, "invalid type conversion")
				}
			}
			newSlice, err := appendSlice(expr, reflect.MakeSlice(lhsT, 0, value.Len()), value)
			if err != nil {
				return nilValue, err
			}
			lhsV = reflect.Append(lhsV, newSlice)
		}
		return lhsV, nil
	}

	return nilValue, newStringError(expr, "invalid type conversion")
}

// nolint: gocyclo
//gocyclo:ignore
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

func getTypeFromEnv(runInfo *runInfoStruct, typeStruct *ast.TypeStruct) reflect.Type {
	var e *env.Env
	e, runInfo.err = runInfo.env.GetEnvFromPath(typeStruct.Env)
	if runInfo.err != nil {
		return nil
	}

	var t reflect.Type
	t, runInfo.err = e.Type(typeStruct.Name)
	return t
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

// precedenceOfKinds returns the greater of two kinds, string > float > int.
func precedenceOfKinds(kind1 reflect.Kind, kind2 reflect.Kind) reflect.Kind {
	if kind1 == kind2 {
		return kind1
	}
	switch kind1 {
	case reflect.String:
		return kind1
	case reflect.Float64, reflect.Float32:
		switch kind2 {
		case reflect.String:
			return kind2
		}
		return kind1
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch kind2 {
		case reflect.String, reflect.Float64, reflect.Float32:
			return kind2
		}
	}
	return kind1
}
