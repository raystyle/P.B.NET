package vm

import (
	"errors"
	"reflect"
)

var (
	errInvalidTypeConversion = errors.New("invalid type conversion")
)

// convertReflectValueToType is used to covert the reflect.Value to the reflect.Type
// if it can not, it returns the original rv and an error
func convertReflectValueToType(rv reflect.Value, rt reflect.Type) (reflect.Value, error) {
	return rv, errInvalidTypeConversion
}
