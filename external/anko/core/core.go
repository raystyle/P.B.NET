package core

import (
	"fmt"
	"reflect"

	"project/external/anko/env"
)

// Import defines core language builtins - keys, range, println,  etc.
func Import(e *env.Env) {
	for _, item := range [...]*struct {
		symbol string
		fn     interface{}
	}{
		{"keys", coreKeys},
		{"range", coreRange},
		{"typeOf", coreTypeOf},
		{"kindOf", coreKindOf},
		{"print", fmt.Print},
		{"println", fmt.Println},
		{"printf", fmt.Printf},
	} {
		err := e.Define(item.symbol, item.fn)
		if err != nil {
			panic(fmt.Sprintf("anko: internal error: %s", err))
		}
	}
	ImportToX(e)
}

func coreKeys(v interface{}) []interface{} {
	rv := reflect.ValueOf(v)
	keysValue := rv.MapKeys()
	keys := make([]interface{}, len(keysValue))
	for i := 0; i < len(keysValue); i++ {
		keys[i] = keysValue[i].Interface()
	}
	return keys
}

func coreRange(args ...int64) []int64 {
	var (
		start int64
		stop  int64
	)
	step := int64(1)
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
	var val []int64
	for i := start; (step > 0 && i < stop) || (step < 0 && i > stop); i += step {
		val = append(val, i)
	}
	return val
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
