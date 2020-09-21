package vm

import (
	"errors"
	"reflect"
)

var (
	errInvalidTypeConversion = errors.New("invalid type conversion")
)

// reflectValueSliceToInterfaceSlice convert from a slice of reflect.Value to a interface slice
// returned in normal reflect.Value form
func reflectValueSliceToInterfaceSlice(valueSlice []reflect.Value) reflect.Value {
	interfaceSlice := make([]interface{}, 0, len(valueSlice))
	for _, value := range valueSlice {
		if value.Kind() == reflect.Interface && !value.IsNil() {
			value = value.Elem()
		}
		if value.CanInterface() {
			interfaceSlice = append(interfaceSlice, value.Interface())
		} else {
			interfaceSlice = append(interfaceSlice, nil)
		}
	}
	return reflect.ValueOf(interfaceSlice)
}

// convertReflectValueToType is used to covert the reflect.Value to the reflect.Type
// if it can not, it returns the original rv and an error
func convertReflectValueToType(rv reflect.Value, rt reflect.Type) (reflect.Value, error) {
	return rv, errInvalidTypeConversion
}

// convertSliceOrArray is used to covert the reflect.Value slice or array to the slice or array reflect.Type
func convertSliceOrArray(rv reflect.Value, rt reflect.Type) (reflect.Value, error) {
	rtElemType := rt.Elem()

	// try to covert elements to new slice/array
	var value reflect.Value
	if rt.Kind() == reflect.Slice {
		// make slice
		value = reflect.MakeSlice(rt, rv.Len(), rv.Len())
	} else {
		// make array
		value = reflect.New(rt).Elem()
	}

	var err error
	var v reflect.Value
	for i := 0; i < rv.Len(); i++ {
		v, err = convertReflectValueToType(rv.Index(i), rtElemType)
		if err != nil {
			return rv, err
		}
		value.Index(i).Set(v)
	}

	// return new converted slice or array
	return value, nil
}
