package vm

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

var errInvalidTypeConversion = errors.New("invalid type conversion")

// reflectValueSliceToInterfaceSlice convert from a slice of reflect.Value to a
// interface slice returned in normal reflect.Value form.
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
// if it can not, it returns the original rv and an error.
// nolint: gocyclo
//gocyclo:ignore
func convertReflectValueToType(rv reflect.Value, rt reflect.Type) (reflect.Value, error) {
	if rt == interfaceType || rv.Type() == rt {
		// if reflect.Type is interface or the types match, return the provided reflect.Value
		return rv, nil
	}
	if rv.Type().ConvertibleTo(rt) {
		// if reflect can covert, do that conversion and return
		return rv.Convert(rt), nil
	}
	if (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) &&
		(rt.Kind() == reflect.Slice || rt.Kind() == reflect.Array) {
		// covert slice or array
		return convertSliceOrArray(rv, rt)
	}
	if rv.Kind() == rt.Kind() {
		// kind matches
		switch rv.Kind() {
		case reflect.Map:
			// convert map
			return convertMap(rv, rt)
		case reflect.Func:
			// for runVMFunction conversions, call convertVMFunctionToType
			return convertVMFunctionToType(rv, rt)
		case reflect.Ptr:
			// both rv and rt are pointers, convert what they are pointing to
			value, err := convertReflectValueToType(rv.Elem(), rt.Elem())
			if err != nil {
				return rv, err
			}
			// need to make a new value to be able to set it
			ptrV, err := makeValue(rt)
			if err != nil {
				return rv, err
			}
			// set value and return new pointer
			ptrV.Elem().Set(value)
			return ptrV, nil
		}
	}
	if rv.Type() == interfaceType {
		if rv.IsNil() {
			// return nil of correct type
			return reflect.Zero(rt), nil
		}
		// try to convert the element
		return convertReflectValueToType(rv.Elem(), rt)
	}

	if rv.Type() == stringType {
		if rt == byteType {
			aString := rv.String()
			if len(aString) < 1 {
				return reflect.Zero(rt), nil
			}
			if len(aString) > 1 {
				return rv, errInvalidTypeConversion
			}
			return reflect.ValueOf(aString[0]), nil
		}
		if rt == runeType {
			aString := rv.String()
			if len(aString) < 1 {
				return reflect.Zero(rt), nil
			}
			if len(aString) > 1 {
				return rv, errInvalidTypeConversion
			}
			return reflect.ValueOf(rune(aString[0])), nil
		}
	}
	return rv, errInvalidTypeConversion
}

// convertSliceOrArray is used to covert the reflect.Value slice or array to
// the slice or array reflect.Type.
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

// convertVMFunctionToType is for translating a runVMFunction into the correct type
// so it can be passed to a Go function argument with the correct static types
// it creates a translate function runVMConvertFunction
func convertVMFunctionToType(rv reflect.Value, rt reflect.Type) (reflect.Value, error) {
	// only translates runVMFunction type
	if !checkIfRunVMFunction(rv.Type()) {
		return rv, errInvalidTypeConversion
	}

	// create runVMConvertFunction to match reflect.Type
	// this function is being called by the Go function
	runVMConvertFunction := func(in []reflect.Value) []reflect.Value {
		// note: this function is being called by another reflect Call
		// only way to pass along any errors is by panic

		// make the reflect.Value slice of each of the VM reflect.Value
		args := make([]reflect.Value, 0, rt.NumIn()+1)
		// for runVMFunction first arg is always context
		// TO FIX: use normal context
		args = append(args, reflect.ValueOf(context.Background()))
		for i := 0; i < rt.NumIn(); i++ {
			// have to do the double reflect.ValueOf that runVMFunction expects
			args = append(args, reflect.ValueOf(in[i]))
		}

		// Call runVMFunction
		rvs := rv.Call(args)

		// call processCallReturnValues to process runVMFunction return values
		// returns normal VM reflect.Value form
		rv, err := processCallReturnValues(rvs, true, false)
		if err != nil {
			panic(err)
		}

		if rt.NumOut() < 1 {
			// Go function does not want any return values, so give it none
			return []reflect.Value{}
		}
		if rt.NumOut() < 2 {
			// Go function wants one return value
			// will try to covert to reflect.Value correct type and return
			v, err := convertReflectValueToType(rv, rt.Out(0))
			if err != nil {
				const format = "function wants return type %s but received type %s"
				panic(fmt.Sprintf(format, rt.Out(0), rv.Type()))
			}
			return []reflect.Value{v}
		}

		// Go function wants more than one return value
		// make sure we have a slice/array with enough values

		if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
			const format = "function wants %v return values but received %v"
			panic(fmt.Sprintf(format, rt.NumOut(), rv.Kind()))
		}
		if rv.Len() < rt.NumOut() {
			const format = "function wants %v return values but received %v values"
			panic(fmt.Sprintf(format, rt.NumOut(), rv.Len()))
		}

		// try to covert each value in slice to wanted type and put into a reflect.Value slice
		rvs = make([]reflect.Value, rt.NumOut())
		for i := 0; i < rv.Len(); i++ {
			rvs[i], err = convertReflectValueToType(rv.Index(i), rt.Out(i))
			if err != nil {
				const format = "function wants return type %s but received type %s"
				panic(fmt.Sprintf(format, rt.Out(i), rvs[i].Type()))
			}
		}

		// return created reflect.Value slice
		return rvs
	}

	// make the reflect.Value function that calls runVMConvertFunction
	return reflect.MakeFunc(rt, runVMConvertFunction), nil
}
