package vm

import (
	"fmt"
	"reflect"
)

// checkIfRunVMFunction checking the number and types of the reflect.Type.
// If it matches the types for a runVMFunction this will return true, otherwise false.
func checkIfRunVMFunction(rt reflect.Type) bool {
	if rt.NumIn() < 1 || rt.NumOut() != 2 || rt.In(0) != contextType ||
		rt.Out(0) != reflectValueType || rt.Out(1) != reflectValueType {
		return false
	}
	if rt.NumIn() > 1 {
		if rt.IsVariadic() {
			if rt.In(rt.NumIn()-1) != interfaceSliceType {
				return false
			}
		} else {
			if rt.In(rt.NumIn()-1) != reflectValueType {
				return false
			}
		}
		for i := 1; i < rt.NumIn()-1; i++ {
			if rt.In(i) != reflectValueType {
				return false
			}
		}
	}
	return true
}

// processCallReturnValues get/converts the values returned from a function call
// into our normal reflect.Value, error.
func processCallReturnValues(rvs []reflect.Value, runVMFunc, toInterfaceSlice bool) (reflect.Value, error) {
	// check if it is not runVMFunction
	if !runVMFunc {
		// the function was a Go function, convert to our normal reflect.Value, error
		switch len(rvs) {
		case 0:
			// no return values so return nil reflect.Value and nil error
			return nilValue, nil
		case 1:
			// one return value but need to add nil error
			return rvs[0], nil
		}
		if toInterfaceSlice {
			// need to convert from a slice of reflect.Value to slice of interface
			return reflectValueSliceToInterfaceSlice(rvs), nil
		}
		// need to keep as slice of reflect.Value
		return reflect.ValueOf(rvs), nil
	}

	// is a runVMFunction, expect return in the runVMFunction format
	// convertToInterfaceSlice is ignored
	// some of the below checks probably can be removed because they are done in checkIfRunVMFunction

	if len(rvs) != 2 {
		const format = "VM function did not return 2 values but returned %v values"
		return nilValue, fmt.Errorf(format, len(rvs))
	}
	if rvs[0].Type() != reflectValueType {
		const format = "VM function value 1 did not return reflect value type but returned %v type"
		return nilValue, fmt.Errorf(format, rvs[0].Type().String())
	}
	if rvs[1].Type() != reflectValueType {
		const format = "VM function value 2 did not return reflect value type but returned %v type"
		return nilValue, fmt.Errorf(format, rvs[1].Type().String())
	}

	rvError := rvs[1].Interface().(reflect.Value)
	if rvError.Type() != errorType && rvError.Type() != vmErrorType {
		return nilValue, fmt.Errorf("VM function error type is %v", rvError.Type())
	}

	if rvError.IsNil() {
		// no error, so return the normal VM reflect.Value form
		return rvs[0].Interface().(reflect.Value), nil
	}

	// VM returns two types of errors, check to see which type
	if rvError.Type() == vmErrorType {
		// convert to VM *Error
		return nilValue, rvError.Interface().(*Error)
	}
	// convert to error
	return nilValue, rvError.Interface().(error)
}
