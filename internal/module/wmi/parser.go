// +build windows

package wmi

import (
	"fmt"
	"reflect"

	"project/internal/xpanic"
)

// parseResult is used to parse ExecQuery and ExecMethod result to destination interface.
func parseResult(objects []*Object, dst interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "parseResult")
		}
	}()
	// walk destination structure for get structure fields
	val := reflect.ValueOf(dst)
	typ := checkDstType(dst, val)
	// set values
	objectsLen := len(objects)

	fmt.Println(val)
	fmt.Println(typ)

	val.Set(reflect.MakeSlice(typ, objectsLen, objectsLen))

	for i := 0; i < objectsLen; i++ {
		name, err := objects[i].GetProperty("name")
		if err != nil {
			return err
		}
		name.Clear()

	}

	return
}

func checkDstType(dst interface{}, val reflect.Value) reflect.Type {
	typ := reflect.TypeOf(dst)
	if typ.Kind() != reflect.Ptr || val.IsNil() {
		panic("destination interface is not slice pointer or is nil")
	}
	slice := typ.Elem()
	if slice.Kind() != reflect.Slice {
		panic("destination pointer is not point to slice")
	}
	elemType := slice.Elem()
	switch elemType.Kind() {
	case reflect.Struct:
	case reflect.Ptr:
		if elemType.Elem().Kind() != reflect.Struct {
			panic("destination slice element pointer is not point to structure")
		}
	default:
		panic("destination slice element is not structure or pointer")
	}
	return slice
}
