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
		"Convert":          reflect.ValueOf(convert),
		"ConvertWithType":  reflect.ValueOf(convertWithType),
		"ArrayOf":          reflect.ValueOf(arrayOf),
		"ArrayToSlice":     reflect.ValueOf(arrayToSlice),
		"ByteArrayToSlice": reflect.ValueOf(byteArrayToSlice),
		"Sizeof":           reflect.ValueOf(sizeOf),
		"Alignof":          reflect.ValueOf(alignOf),
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
func convert(point *interface{}, typ interface{}) reflect.Value {
	return convertWithType(point, reflect.TypeOf(typ))
}

func convertWithType(point *interface{}, typ reflect.Type) reflect.Value {
	ptr := reflect.ValueOf(point).Elem().InterfaceData()[1]
	return reflect.NewAt(typ, unsafe.Pointer(ptr)).Elem() // #nosec
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

func sizeOf(i interface{}) uintptr {
	return reflect.ValueOf(i).Type().Size()
}

func alignOf(i interface{}) uintptr {
	return uintptr(reflect.ValueOf(i).Type().Align())
}
