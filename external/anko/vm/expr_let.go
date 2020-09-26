package vm

import (
	"fmt"
	"reflect"

	"project/external/anko/ast"
	"project/external/anko/env"
)

// nolint: gocyclo
//gocyclo:ignore
func (runInfo *runInfoStruct) invokeLetExpr() {
	switch expr := runInfo.expr.(type) {

	// IdentExpr
	case *ast.IdentExpr:
		if runInfo.env.SetValue(expr.Lit, runInfo.rv) != nil {
			runInfo.err = nil
			_ = runInfo.env.DefineValue(expr.Lit, runInfo.rv)
		}

	// MemberExpr
	case *ast.MemberExpr:
		value := runInfo.rv

		runInfo.expr = expr.Expr
		runInfo.invokeExpr()
		if runInfo.err != nil {
			return
		}

		if runInfo.rv.Kind() == reflect.Interface && !runInfo.rv.IsNil() {
			runInfo.rv = runInfo.rv.Elem()
		}

		if e, ok := runInfo.rv.Interface().(*env.Env); ok {
			runInfo.err = e.SetValue(expr.Name, value)
			if runInfo.err != nil {
				runInfo.err = newError(expr, runInfo.err)
				runInfo.rv = nilValue
			}
			return
		}

		if runInfo.rv.Kind() == reflect.Ptr {
			runInfo.rv = runInfo.rv.Elem()
		}

		switch runInfo.rv.Kind() {

		// Struct
		case reflect.Struct:
			field, found := runInfo.rv.Type().FieldByName(expr.Name)
			if !found {
				errStr := "no member named \"" + expr.Name + "\" for struct"
				runInfo.err = newStringError(expr, errStr)
				runInfo.rv = nilValue
				return
			}
			runInfo.rv = runInfo.rv.FieldByIndex(field.Index)
			// From reflect CanSet:
			// A Value can be changed only if it is addressable and was not obtained by
			// the use of unexported struct fields.
			// Often a struct has to be passed as a pointer to be set
			if !runInfo.rv.CanSet() {
				errStr := "struct member \"" + expr.Name + "\" cannot be assigned"
				runInfo.err = newStringError(expr, errStr)
				runInfo.rv = nilValue
				return
			}

			value, runInfo.err = convertReflectValueToType(value, runInfo.rv.Type())
			if runInfo.err != nil {
				const format = "type %s cannot be assigned to type %s for struct"
				errStr := fmt.Sprintf(format, value.Type(), runInfo.rv.Type())
				runInfo.err = newStringError(expr, errStr)
				runInfo.rv = nilValue
				return
			}

			runInfo.rv.Set(value)
			return

		// Map
		case reflect.Map:
			value, runInfo.err = convertReflectValueToType(value, runInfo.rv.Type().Elem())
			if runInfo.err != nil {
				const format = "type %s cannot be assigned to type %s for map"
				errStr := fmt.Sprintf(format, value.Type(), runInfo.rv.Type().Elem())
				runInfo.err = newStringError(expr, errStr)
				runInfo.rv = nilValue
				return
			}
			if runInfo.rv.IsNil() {
				// make new map
				item := reflect.MakeMap(runInfo.rv.Type())
				item.SetMapIndex(reflect.ValueOf(expr.Name), value)
				// assign new map
				runInfo.rv = item
				runInfo.expr = expr.Expr
				runInfo.invokeLetExpr()
				runInfo.rv = item.MapIndex(reflect.ValueOf(expr.Name))
				return
			}
			runInfo.rv.SetMapIndex(reflect.ValueOf(expr.Name), value)

		default:
			const format = "type %s does not support member operation"
			errStr := fmt.Sprintf(format, +runInfo.rv.Kind())
			runInfo.err = newStringError(expr, errStr)
			runInfo.rv = nilValue
		}

	// ItemExpr
	case *ast.ItemExpr:
		value := runInfo.rv

		runInfo.expr = expr.Item
		runInfo.invokeExpr()
		if runInfo.err != nil {
			return
		}
		item := runInfo.rv

		runInfo.expr = expr.Index
		runInfo.invokeExpr()
		if runInfo.err != nil {
			return
		}

		if item.Kind() == reflect.Interface && !item.IsNil() {
			item = item.Elem()
		}

		switch item.Kind() {

		// Slice && Array
		case reflect.Slice, reflect.Array:
			var index int
			index, runInfo.err = tryToInt(runInfo.rv)
			if runInfo.err != nil {
				runInfo.err = newStringError(expr, "index must be a number")
				runInfo.rv = nilValue
				return
			}

			if index == item.Len() {
				// try to do automatic append
				value, runInfo.err = convertReflectValueToType(value, item.Type().Elem())
				if runInfo.err != nil {
					const format = "type %s cannot be assigned to type %s for slice index"
					errStr := fmt.Sprintf(format, value.Type(), item.Type().Elem())
					runInfo.err = newStringError(expr, errStr)
					runInfo.rv = nilValue
					return
				}
				item = reflect.Append(item, value)
				runInfo.rv = item
				runInfo.expr = expr.Item
				runInfo.invokeLetExpr()
				runInfo.rv = item.Index(index)
				return
			}

			if index < 0 || index >= item.Len() {
				runInfo.err = newStringError(expr, "index out of range")
				runInfo.rv = nilValue
				return
			}
			item = item.Index(index)
			if !item.CanSet() {
				runInfo.err = newStringError(expr, "index cannot be assigned")
				runInfo.rv = nilValue
				return
			}

			value, runInfo.err = convertReflectValueToType(value, item.Type())
			if runInfo.err != nil {
				const format = "type %s cannot be assigned to type %s for slice index"
				errStr := fmt.Sprintf(format, value.Type(), item.Type())
				runInfo.err = newStringError(expr, errStr)
				runInfo.rv = nilValue
				return
			}

			item.Set(value)
			runInfo.rv = item

		// Map
		case reflect.Map:
			runInfo.rv, runInfo.err = convertReflectValueToType(runInfo.rv, item.Type().Key())
			if runInfo.err != nil {
				const format = "index type %s cannot be used for map index type %s"
				errStr := fmt.Sprintf(format, runInfo.rv.Type(), item.Type().Key())
				runInfo.err = newStringError(expr, errStr)
				runInfo.rv = nilValue
				return
			}

			value, runInfo.err = convertReflectValueToType(value, item.Type().Elem())
			if runInfo.err != nil {
				const format = "type %s cannot be assigned to type %s for map"
				errStr := fmt.Sprintf(format, value.Type(), item.Type().Elem())
				runInfo.err = newStringError(expr, errStr)
				runInfo.rv = nilValue
				return
			}

			if item.IsNil() {
				// make new map
				item = reflect.MakeMap(item.Type())
				item.SetMapIndex(runInfo.rv, value)
				mapIndex := runInfo.rv
				// assign new map
				runInfo.rv = item
				runInfo.expr = expr.Item
				runInfo.invokeLetExpr()
				runInfo.rv = item.MapIndex(mapIndex)
				return
			}
			item.SetMapIndex(runInfo.rv, value)

		// String
		case reflect.String:
			var index int
			index, runInfo.err = tryToInt(runInfo.rv)
			if runInfo.err != nil {
				runInfo.err = newStringError(expr, "index must be a number")
				runInfo.rv = nilValue
				return
			}

			value, runInfo.err = convertReflectValueToType(value, item.Type())
			if runInfo.err != nil {
				const format = "type %s cannot be assigned to type %s"
				errStr := fmt.Sprintf(format, value.Type(), item.Type())
				runInfo.err = newStringError(expr, errStr)
				runInfo.rv = nilValue
				return
			}

			if index == item.Len() {
				// automatic append
				if item.CanSet() {
					item.SetString(item.String() + value.String())
					return
				}

				runInfo.rv = reflect.ValueOf(item.String() + value.String())
				runInfo.expr = expr.Item
				runInfo.invokeLetExpr()
				return
			}

			if index < 0 || index >= item.Len() {
				runInfo.err = newStringError(expr, "index out of range")
				runInfo.rv = nilValue
				return
			}

			if item.CanSet() {
				str := item.Slice(0, index).String() + value.String() +
					item.Slice(index+1, item.Len()).String()
				item.SetString(str)
				runInfo.rv = item
				return
			}
			str := item.Slice(0, index).String() + value.String() +
				item.Slice(index+1, item.Len()).String()
			runInfo.rv = reflect.ValueOf(str)
			runInfo.expr = expr.Item
			runInfo.invokeLetExpr()

		default:
			const format = "type %s does not support index operation"
			errStr := fmt.Sprintf(format, item.Kind())
			runInfo.err = newStringError(expr, errStr)
			runInfo.rv = nilValue
		}

	// dereference expr
	case *ast.DerefExpr:
		value := runInfo.rv

		runInfo.expr = expr.Expr
		runInfo.invokeExpr()
		if runInfo.err != nil {
			return
		}

		runInfo.rv.Elem().Set(value)
		runInfo.rv = value

	default:
		runInfo.err = newStringError(expr, "invalid operation")
		runInfo.rv = nilValue
	}
}
