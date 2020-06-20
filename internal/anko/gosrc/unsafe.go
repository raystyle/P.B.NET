package gosrc

import (
	"reflect"
	"unsafe"

	"github.com/mattn/anko/env"
)

func init() {
	initUnsafe()
}

func initUnsafe() {
	env.Packages["unsafe"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"Sizeof":  reflect.ValueOf(sizeOf),
		"Alignof": reflect.ValueOf(alignOf),
	}
	var (
		pointer unsafe.Pointer
	)
	env.PackageTypes["unsafe"] = map[string]reflect.Type{
		"Pointer": reflect.TypeOf(&pointer).Elem(),
	}
}

func sizeOf(i interface{}) uintptr {
	return reflect.ValueOf(i).Type().Size()
}

func alignOf(i interface{}) uintptr {
	return uintptr(reflect.ValueOf(i).Type().Align())
}
