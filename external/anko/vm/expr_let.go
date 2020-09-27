package vm

import (
	"fmt"
	"reflect"

	"project/external/anko/ast"
	"project/external/anko/env"
)

// nolint: gocyclo
//gocyclo:ignore
func (ri *runInfo) invokeLetExpr() {
	switch expr := ri.expr.(type) {

	// IdentExpr
	case *ast.IdentExpr:
		if ri.env.SetValue(expr.Lit, ri.rv) != nil {
			ri.err = nil
			_ = ri.env.DefineValue(expr.Lit, ri.rv)
		}

	// MemberExpr
	case *ast.MemberExpr:
		value := ri.rv

		ri.expr = expr.Expr
		ri.invokeExpr()
		if ri.err != nil {
			return
		}

		if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
			ri.rv = ri.rv.Elem()
		}

		if e, ok := ri.rv.Interface().(*env.Env); ok {
			ri.err = e.SetValue(expr.Name, value)
			if ri.err != nil {
				ri.err = newError(expr, ri.err)
				ri.rv = nilValue
			}
			return
		}

		if ri.rv.Kind() == reflect.Ptr {
			ri.rv = ri.rv.Elem()
		}

		switch ri.rv.Kind() {

		// Struct
		case reflect.Struct:
			field, found := ri.rv.Type().FieldByName(expr.Name)
			if !found {
				errStr := "no member named \"" + expr.Name + "\" for struct"
				ri.err = newStringError(expr, errStr)
				ri.rv = nilValue
				return
			}
			ri.rv = ri.rv.FieldByIndex(field.Index)
			// From reflect CanSet:
			// A Value can be changed only if it is addressable and was not obtained by
			// the use of unexported struct fields.
			// Often a struct has to be passed as a pointer to be set
			if !ri.rv.CanSet() {
				errStr := "struct member \"" + expr.Name + "\" cannot be assigned"
				ri.err = newStringError(expr, errStr)
				ri.rv = nilValue
				return
			}

			value, ri.err = convertReflectValueToType(value, ri.rv.Type())
			if ri.err != nil {
				const format = "type %s cannot be assigned to type %s for struct"
				errStr := fmt.Sprintf(format, value.Type(), ri.rv.Type())
				ri.err = newStringError(expr, errStr)
				ri.rv = nilValue
				return
			}

			ri.rv.Set(value)
			return

		// Map
		case reflect.Map:
			value, ri.err = convertReflectValueToType(value, ri.rv.Type().Elem())
			if ri.err != nil {
				const format = "type %s cannot be assigned to type %s for map"
				errStr := fmt.Sprintf(format, value.Type(), ri.rv.Type().Elem())
				ri.err = newStringError(expr, errStr)
				ri.rv = nilValue
				return
			}
			if ri.rv.IsNil() {
				// make new map
				item := reflect.MakeMap(ri.rv.Type())
				item.SetMapIndex(reflect.ValueOf(expr.Name), value)
				// assign new map
				ri.rv = item
				ri.expr = expr.Expr
				ri.invokeLetExpr()
				ri.rv = item.MapIndex(reflect.ValueOf(expr.Name))
				return
			}
			ri.rv.SetMapIndex(reflect.ValueOf(expr.Name), value)

		default:
			const format = "type %s does not support member operation"
			errStr := fmt.Sprintf(format, +ri.rv.Kind())
			ri.err = newStringError(expr, errStr)
			ri.rv = nilValue
		}

	// ItemExpr
	case *ast.ItemExpr:
		value := ri.rv

		ri.expr = expr.Item
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		item := ri.rv

		ri.expr = expr.Index
		ri.invokeExpr()
		if ri.err != nil {
			return
		}

		if item.Kind() == reflect.Interface && !item.IsNil() {
			item = item.Elem()
		}

		switch item.Kind() {

		// Slice && Array
		case reflect.Slice, reflect.Array:
			var index int
			index, ri.err = tryToInt(ri.rv)
			if ri.err != nil {
				ri.err = newStringError(expr, "index must be a number")
				ri.rv = nilValue
				return
			}

			if index == item.Len() {
				// try to do automatic append
				value, ri.err = convertReflectValueToType(value, item.Type().Elem())
				if ri.err != nil {
					const format = "type %s cannot be assigned to type %s for slice index"
					errStr := fmt.Sprintf(format, value.Type(), item.Type().Elem())
					ri.err = newStringError(expr, errStr)
					ri.rv = nilValue
					return
				}
				item = reflect.Append(item, value)
				ri.rv = item
				ri.expr = expr.Item
				ri.invokeLetExpr()
				ri.rv = item.Index(index)
				return
			}

			if index < 0 || index >= item.Len() {
				ri.err = newStringError(expr, "index out of range")
				ri.rv = nilValue
				return
			}
			item = item.Index(index)
			if !item.CanSet() {
				ri.err = newStringError(expr, "index cannot be assigned")
				ri.rv = nilValue
				return
			}

			value, ri.err = convertReflectValueToType(value, item.Type())
			if ri.err != nil {
				const format = "type %s cannot be assigned to type %s for slice index"
				errStr := fmt.Sprintf(format, value.Type(), item.Type())
				ri.err = newStringError(expr, errStr)
				ri.rv = nilValue
				return
			}

			item.Set(value)
			ri.rv = item

		// Map
		case reflect.Map:
			ri.rv, ri.err = convertReflectValueToType(ri.rv, item.Type().Key())
			if ri.err != nil {
				const format = "index type %s cannot be used for map index type %s"
				errStr := fmt.Sprintf(format, ri.rv.Type(), item.Type().Key())
				ri.err = newStringError(expr, errStr)
				ri.rv = nilValue
				return
			}

			value, ri.err = convertReflectValueToType(value, item.Type().Elem())
			if ri.err != nil {
				const format = "type %s cannot be assigned to type %s for map"
				errStr := fmt.Sprintf(format, value.Type(), item.Type().Elem())
				ri.err = newStringError(expr, errStr)
				ri.rv = nilValue
				return
			}

			if item.IsNil() {
				// make new map
				item = reflect.MakeMap(item.Type())
				item.SetMapIndex(ri.rv, value)
				mapIndex := ri.rv
				// assign new map
				ri.rv = item
				ri.expr = expr.Item
				ri.invokeLetExpr()
				ri.rv = item.MapIndex(mapIndex)
				return
			}
			item.SetMapIndex(ri.rv, value)

		// String
		case reflect.String:
			var index int
			index, ri.err = tryToInt(ri.rv)
			if ri.err != nil {
				ri.err = newStringError(expr, "index must be a number")
				ri.rv = nilValue
				return
			}

			value, ri.err = convertReflectValueToType(value, item.Type())
			if ri.err != nil {
				const format = "type %s cannot be assigned to type %s"
				errStr := fmt.Sprintf(format, value.Type(), item.Type())
				ri.err = newStringError(expr, errStr)
				ri.rv = nilValue
				return
			}

			if index == item.Len() {
				// automatic append
				if item.CanSet() {
					item.SetString(item.String() + value.String())
					return
				}

				ri.rv = reflect.ValueOf(item.String() + value.String())
				ri.expr = expr.Item
				ri.invokeLetExpr()
				return
			}

			if index < 0 || index >= item.Len() {
				ri.err = newStringError(expr, "index out of range")
				ri.rv = nilValue
				return
			}

			if item.CanSet() {
				str := item.Slice(0, index).String() + value.String() +
					item.Slice(index+1, item.Len()).String()
				item.SetString(str)
				ri.rv = item
				return
			}
			str := item.Slice(0, index).String() + value.String() +
				item.Slice(index+1, item.Len()).String()
			ri.rv = reflect.ValueOf(str)
			ri.expr = expr.Item
			ri.invokeLetExpr()

		default:
			const format = "type %s does not support index operation"
			errStr := fmt.Sprintf(format, item.Kind())
			ri.err = newStringError(expr, errStr)
			ri.rv = nilValue
		}

	// SliceExpr
	case *ast.SliceExpr:
		value := ri.rv

		ri.expr = expr.Item
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		item := ri.rv

		if item.Kind() == reflect.Interface && !item.IsNil() {
			item = item.Elem()
		}

		switch item.Kind() {

		// Slice && Array
		case reflect.Slice, reflect.Array:
			var beginIndex int
			endIndex := item.Len()

			if expr.Begin != nil {
				ri.expr = expr.Begin
				ri.invokeExpr()
				if ri.err != nil {
					return
				}
				beginIndex, ri.err = tryToInt(ri.rv)
				if ri.err != nil {
					ri.err = newStringError(expr, "index must be a number")
					ri.rv = nilValue
					return
				}
				// (0 <= low) <= high <= len(a)
				if beginIndex < 0 {
					ri.err = newStringError(expr, "index out of range")
					ri.rv = nilValue
					return
				}
			}

			if expr.End != nil {
				ri.expr = expr.End
				ri.invokeExpr()
				if ri.err != nil {
					return
				}
				endIndex, ri.err = tryToInt(ri.rv)
				if ri.err != nil {
					ri.err = newStringError(expr, "index must be a number")
					ri.rv = nilValue
					return
				}
				// 0 <= low <= (high <= len(a))
				if endIndex > item.Len() {
					ri.err = newStringError(expr, "index out of range")
					ri.rv = nilValue
					return
				}
			}

			// 0 <= (low <= high) <= len(a)
			if beginIndex > endIndex {
				ri.err = newStringError(expr, "index out of range")
				ri.rv = nilValue
				return
			}

			sliceCap := item.Cap()
			if expr.Cap != nil {
				ri.expr = expr.Cap
				ri.invokeExpr()
				if ri.err != nil {
					return
				}
				sliceCap, ri.err = tryToInt(ri.rv)
				if ri.err != nil {
					ri.err = newStringError(expr, "cap must be a number")
					ri.rv = nilValue
					return
				}
				//  0 <= low <= (high <= max <= cap(a))
				if sliceCap < endIndex || sliceCap > item.Cap() {
					ri.err = newStringError(expr, "cap out of range")
					ri.rv = nilValue
					return
				}
			}

			item = item.Slice3(beginIndex, endIndex, sliceCap)

			if !item.CanSet() {
				ri.err = newStringError(expr, "slice cannot be assigned")
				ri.rv = nilValue
				return
			}
			item.Set(value)

		// String
		case reflect.String:
			const errStr = "type string does not support slice operation for assignment"
			ri.err = newStringError(expr, errStr)
			ri.rv = nilValue

		default:
			errStr := "type " + item.Kind().String() + " does not support slice operation"
			ri.err = newStringError(expr, errStr)
			ri.rv = nilValue
		}

	// dereference expr
	case *ast.DerefExpr:
		value := ri.rv

		ri.expr = expr.Expr
		ri.invokeExpr()
		if ri.err != nil {
			return
		}

		ri.rv.Elem().Set(value)
		ri.rv = value

	default:
		ri.err = newStringError(expr, "invalid operation")
		ri.rv = nilValue
	}
}
