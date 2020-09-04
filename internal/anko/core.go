package anko

import (
	"fmt"
	"reflect"

	"github.com/mattn/anko/env"
	"github.com/mattn/anko/vm"
)

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

	// "ArrayOf":          reflect.ValueOf(arrayOf),
	// "ArrayToSlice":     reflect.ValueOf(arrayToSlice),
	// "ByteArrayToSlice": reflect.ValueOf(byteArrayToSlice),

	_ = e.Define("kindOf", func(v interface{}) string {
		typeOf := reflect.TypeOf(v)
		if typeOf == nil {
			return "nil"
		}
		return typeOf.Kind().String()
	})

	// code in eval can't  access parent vm
	childEnv := e.DeepCopy()
	_ = e.Define("eval", func(src string) interface{} {
		return coreEval(childEnv, src)
	})
}

// arrayOf will not create a point about array.
func arrayOf(typ interface{}, size int) reflect.Type {
	return reflect.ArrayOf(size, reflect.TypeOf(typ))
}

func arrayToSlice(array reflect.Value) reflect.Value {
	return array.Slice(0, array.Len())
}

func byteArrayToSlice(array reflect.Value) []byte {
	return arrayToSlice(array).Bytes()
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
