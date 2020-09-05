package anko

import (
	"fmt"
	"reflect"

	"github.com/mattn/anko/env"
)

func defineCoreType(e *env.Env) {
	for _, item := range [...]*struct {
		symbol string
		typ    interface{}
	}{
		{"int8", int8(1)},
		{"int16", int16(1)},
		{"uint8", uint8(1)},
		{"uint16", uint16(1)},
		{"uintptr", uintptr(1)},
	} {
		err := e.DefineType(item.symbol, item.typ)
		if err != nil {
			panic(fmt.Sprintf("anko: internal error: %s", err))
		}
	}
}

// defineCoreFunc is used to add core function.
// core.Import() with leaks, so we implement it self.
func defineCoreFunc(e *env.Env) {
	for _, item := range [...]*struct {
		symbol string
		fn     interface{}
	}{
		{"keys", coreKeys},
		{"range", coreRange},
		{"arrayType", coreArrayType},
		{"array", coreArray},
		{"slice", coreSlice},

		{"typeOf", coreTypeOf},
		{"kindOf", coreKindOf},
	} {
		err := e.Define(item.symbol, item.fn)
		if err != nil {
			panic(fmt.Sprintf("anko: internal error: %s", err))
		}
	}
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

// coreArrayType is used to create a array type like [8]byte.
func coreArrayType(typ interface{}, size int) reflect.Type {
	return reflect.ArrayOf(size, reflect.TypeOf(typ))
}

// coreArray is used to create a array like [8]byte{}.
func coreArray(typ interface{}, size int) reflect.Value {
	return reflect.New(coreArrayType(typ, size))
	// return reflect.ValueOf(ptr).Elem().Interface()
}

// coreSlice is used to convert array to slice like [8]byte[:]
func coreSlice(array reflect.Value) interface{} {
	return array.Slice(0, array.Len()).Interface()
}

func coreTypeOf(v interface{}) string {
	return reflect.TypeOf(v).String()
}

func coreKindOf(v interface{}) string {
	typeOf := reflect.TypeOf(v)
	if typeOf == nil {
		return "nil kind"
	}
	return typeOf.Kind().String()
}
