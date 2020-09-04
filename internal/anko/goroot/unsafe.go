package goroot

import (
	"reflect"
	"runtime"
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
		"Convert":         reflect.ValueOf(convert),
		"ConvertWithType": reflect.ValueOf(convertWithType),
		"Sizeof":          reflect.ValueOf(sizeOf),
		"Alignof":         reflect.ValueOf(alignOf),
	}
	var (
		pointer unsafe.Pointer
	)
	env.PackageTypes["unsafe"] = map[string]reflect.Type{
		"Pointer": reflect.TypeOf(&pointer).Elem(),
	}
}

// convert is used to force convert like
// n := *(*uint32)(unsafe.Pointer(&Float32))
//
// you can use these code in anko script
//
// p = unsafe.Convert(&val, new(typ))
// println(p.Interface().A)          // get value in struct
// p.Set(reflect.ValueOf(newVal))    // set value
//
// newVal must the same type with typ
// see more information in TestUnsafe()
func convert(pointer *interface{}, typ interface{}) interface{} {
	return convertWithType(pointer, reflect.TypeOf(typ))
}

//go:nocheckptr
func convertWithType(pointer *interface{}, typ reflect.Type) interface{} {
	address := reflect.ValueOf(pointer).Elem().InterfaceData()[1]
	ptr := reflect.NewAt(typ, unsafe.Pointer(address)).Interface() // #nosec
	runtime.KeepAlive(pointer)
	return ptr
}

func sizeOf(i interface{}) uintptr {
	return reflect.ValueOf(i).Type().Size()
}

func alignOf(i interface{}) uintptr {
	return uintptr(reflect.ValueOf(i).Type().Align())
}
