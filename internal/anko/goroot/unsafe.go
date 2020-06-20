package goroot

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
		"Convert": reflect.ValueOf(convert),
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

func convert(from, to reflect.Type) func(src, dest reflect.Value) {
	switch {
	case to.Kind() == reflect.UnsafePointer && from.Kind() == reflect.Uintptr:
		return uintptrToUnsafePtr
	case to.Kind() == reflect.UnsafePointer:
		return func(src, dest reflect.Value) {
			dest.SetPointer(unsafe.Pointer(src.Pointer())) // #nosec
		}
	case to.Kind() == reflect.Uintptr && from.Kind() == reflect.UnsafePointer:
		return func(src, dest reflect.Value) {
			ptr := src.Interface().(unsafe.Pointer)
			dest.Set(reflect.ValueOf(uintptr(ptr)))
		}
	case from.Kind() == reflect.UnsafePointer:
		return func(src, dest reflect.Value) {
			ptr := src.Interface().(unsafe.Pointer)
			v := reflect.NewAt(dest.Type().Elem(), ptr)
			dest.Set(v)
		}
	default:
		return nil
	}
}

//go:nocheckptr
func uintptrToUnsafePtr(src, dest reflect.Value) {
	dest.SetPointer(unsafe.Pointer(src.Interface().(uintptr))) // #nosec
}

func sizeOf(i interface{}) uintptr {
	return reflect.ValueOf(i).Type().Size()
}

func alignOf(i interface{}) uintptr {
	return uintptr(reflect.ValueOf(i).Type().Align())
}
