// +build windows

package wmi

import (
	"reflect"

	"project/internal/xpanic"
)

func parseExecQueryResult(objects []*Object, dst interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "parseExecQueryResult")
		}
	}()
	// walk destination structure for get structure fields
	typ := reflect.TypeOf(dst)
	if typ.Kind() != reflect.Slice {
		panic("destination interface is not slice")
	}
	typ = typ.Elem()
	switch typ.Kind() {
	case reflect.Struct:
	case reflect.Ptr:
		typ = typ.Elem()
		if typ.Kind() != reflect.Struct {
			panic("destination slice element point is not point to structure")
		}
	default:
		panic("destination slice element is not structure or point")
	}

	// name, err := objects[i].GetProperty("name")
	// if err != nil {
	// 	return
	// }
	// name.Clear()

	return
}
