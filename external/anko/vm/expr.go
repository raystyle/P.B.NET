package vm

import (
	"fmt"
	"reflect"

	"project/external/anko/ast"
	"project/external/anko/env"
)

// invokeExpr evaluates one expression.
// nolint: gocyclo
//gocyclo:ignore
func (ri *runInfo) invokeExpr() {
	switch expr := ri.expr.(type) {

	// OpExpr
	case *ast.OpExpr:
		ri.op = expr.Op
		ri.invokeOperator()

	// IdentExpr
	case *ast.IdentExpr:
		ri.rv, ri.err = ri.env.GetValue(expr.Lit)
		if ri.err != nil {
			ri.err = newError(expr, ri.err)
		}

	// LiteralExpr
	case *ast.LiteralExpr:
		ri.rv = expr.Literal

	// ArrayExpr
	case *ast.ArrayExpr:
		if expr.TypeData == nil {
			slice := make([]interface{}, len(expr.Exprs))
			var i int
			for i, ri.expr = range expr.Exprs {
				ri.invokeExpr()
				if ri.err != nil {
					return
				}
				slice[i] = ri.rv.Interface()
			}
			ri.rv = reflect.ValueOf(slice)
			return
		}

		t := makeType(ri, expr.TypeData)
		if ri.err != nil {
			ri.rv = nilValue
			return
		}
		if t == nil {
			ri.err = newStringError(expr, "cannot make type nil")
			ri.rv = nilValue
			return
		}

		slice := reflect.MakeSlice(t, len(expr.Exprs), len(expr.Exprs))
		var i int
		valueType := t.Elem()
		for i, ri.expr = range expr.Exprs {
			ri.invokeExpr()
			if ri.err != nil {
				return
			}

			ri.rv, ri.err = convertReflectValueToType(ri.rv, valueType)
			if ri.err != nil {
				const format = "cannot use type %s as type %s as slice value"
				errStr := fmt.Sprintf(format, ri.rv.Type(), valueType)
				ri.err = newStringError(expr, errStr)
				ri.rv = nilValue
				return
			}

			slice.Index(i).Set(ri.rv)
		}
		ri.rv = slice

		// MapExpr
	case *ast.MapExpr:
		if expr.TypeData == nil {
			var i int
			var key reflect.Value
			m := make(map[interface{}]interface{}, len(expr.Keys))
			for i, ri.expr = range expr.Keys {
				ri.invokeExpr()
				if ri.err != nil {
					return
				}
				key = ri.rv

				ri.expr = expr.Values[i]
				ri.invokeExpr()
				if ri.err != nil {
					return
				}

				m[key.Interface()] = ri.rv.Interface()
			}
			ri.rv = reflect.ValueOf(m)
			return
		}

		t := makeType(ri, expr.TypeData)
		if ri.err != nil {
			ri.rv = nilValue
			return
		}
		if t == nil {
			ri.err = newStringError(expr, "cannot make type nil")
			ri.rv = nilValue
			return
		}

		ri.rv, ri.err = makeValue(t)
		if ri.err != nil {
			ri.rv = nilValue
			return
		}

		var i int
		var key reflect.Value
		m := ri.rv
		keyType := t.Key()
		valueType := t.Elem()
		for i, ri.expr = range expr.Keys {
			ri.invokeExpr()
			if ri.err != nil {
				return
			}
			key, ri.err = convertReflectValueToType(ri.rv, keyType)
			if ri.err != nil {
				const format = "cannot use type %s as type %s as map key"
				ri.err = newStringError(expr, fmt.Sprintf(format, key.Type(), keyType))
				ri.rv = nilValue
				return
			}

			ri.expr = expr.Values[i]
			ri.invokeExpr()
			if ri.err != nil {
				return
			}
			ri.rv, ri.err = convertReflectValueToType(ri.rv, valueType)
			if ri.err != nil {
				const format = "cannot use type %s as type %s as map value"
				ri.err = newStringError(expr, fmt.Sprintf(format, ri.rv.Type(), valueType))
				ri.rv = nilValue
				return
			}

			m.SetMapIndex(key, ri.rv)
		}
		ri.rv = m

	// dereferenceExpr
	case *ast.DerefExpr:
		ri.expr = expr.Expr
		ri.invokeExpr()
		if ri.err != nil {
			return
		}

		if ri.rv.Kind() != reflect.Ptr {
			ri.err = newStringError(expr.Expr, "cannot deference non-pointer")
			ri.rv = nilValue
			return
		}
		ri.rv = ri.rv.Elem()

	// AddrExpr
	case *ast.AddrExpr:
		ri.expr = expr.Expr
		ri.invokeExpr()
		if ri.err != nil {
			return
		}

		if ri.rv.CanAddr() {
			ri.rv = ri.rv.Addr()
		} else {
			i := ri.rv.Interface()
			ri.rv = reflect.ValueOf(&i)
		}

	// UnaryExpr
	case *ast.UnaryExpr:
		ri.expr = expr.Expr
		ri.invokeExpr()
		if ri.err != nil {
			return
		}

		switch expr.Operator {
		case "-":
			switch ri.rv.Kind() {
			case reflect.Int64:
				ri.rv = reflect.ValueOf(-ri.rv.Int())
			case reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int, reflect.Bool:
				ri.rv = reflect.ValueOf(-toInt64(ri.rv))
			case reflect.Float64:
				ri.rv = reflect.ValueOf(-ri.rv.Float())
			default:
				ri.rv = reflect.ValueOf(-toFloat64(ri.rv))
			}
		case "^":
			ri.rv = reflect.ValueOf(^toInt64(ri.rv))
		case "!":
			if toBool(ri.rv) {
				ri.rv = falseValue
			} else {
				ri.rv = trueValue
			}
		default:
			ri.err = newStringError(expr, "unknown operator")
			ri.rv = nilValue
		}

	// ParenExpr
	case *ast.ParenExpr:
		ri.expr = expr.SubExpr
		ri.invokeExpr()
		if ri.err != nil {
			return
		}

	// MemberExpr
	case *ast.MemberExpr:
		ri.expr = expr.Expr
		ri.invokeExpr()
		if ri.err != nil {
			return
		}

		if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
			ri.rv = ri.rv.Elem()
		}

		if e, ok := ri.rv.Interface().(*env.Env); ok {
			ri.rv, ri.err = e.GetValue(expr.Name)
			if ri.err != nil {
				ri.err = newError(expr, ri.err)
				ri.rv = nilValue
			}
			return
		}

		value := ri.rv.MethodByName(expr.Name)
		if value.IsValid() {
			ri.rv = value
			return
		}

		if ri.rv.Kind() == reflect.Ptr {
			ri.rv = ri.rv.Elem()
		}

		switch ri.rv.Kind() {
		case reflect.Struct:
			field, found := ri.rv.Type().FieldByName(expr.Name)
			if found {
				ri.rv = ri.rv.FieldByIndex(field.Index)
				return
			}
			if ri.rv.CanAddr() {
				ri.rv = ri.rv.Addr()
				method, found := ri.rv.Type().MethodByName(expr.Name)
				if found {
					ri.rv = ri.rv.Method(method.Index)
					return
				}
			}
			errStr := fmt.Sprintf("no member named \"%s\" for struct", expr.Name)
			ri.err = newStringError(expr, errStr)
			ri.rv = nilValue
		case reflect.Map:
			ri.rv = getMapIndex(reflect.ValueOf(expr.Name), ri.rv)
		default:
			errStr := fmt.Sprintf("type %s does not support member operation", ri.rv.Kind())
			ri.err = newStringError(expr, errStr)
			ri.rv = nilValue
		}

	// ItemExpr
	case *ast.ItemExpr:
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
		case reflect.String, reflect.Slice, reflect.Array:
			var index int
			index, ri.err = tryToInt(ri.rv)
			if ri.err != nil {
				ri.err = newStringError(expr, "index must be a number")
				ri.rv = nilValue
				return
			}
			if index < 0 || index >= item.Len() {
				ri.err = newStringError(expr, "index out of range")
				ri.rv = nilValue
				return
			}
			if item.Kind() != reflect.String {
				ri.rv = item.Index(index)
			} else {
				// String
				ri.rv = item.Index(index).Convert(stringType)
			}
		case reflect.Map:
			ri.rv = getMapIndex(ri.rv, item)
		default:
			errStr := fmt.Sprintf("type %s does not support index operation", item.Kind())
			ri.err = newStringError(expr, errStr)
			ri.rv = nilValue
		}

	// SliceExpr
	case *ast.SliceExpr:
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
		case reflect.String, reflect.Slice, reflect.Array:
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

			if item.Kind() == reflect.String {
				if expr.Cap != nil {
					ri.err = newStringError(expr, "type string does not support cap")
					ri.rv = nilValue
					return
				}
				ri.rv = item.Slice(beginIndex, endIndex)
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

			ri.rv = item.Slice3(beginIndex, endIndex, sliceCap)
		default:
			errStr := fmt.Sprintf("type %s does not support slice operation", item.Kind())
			ri.err = newStringError(expr, errStr)
			ri.rv = nilValue
		}

	// LetsExpr
	case *ast.LetsExpr:
		var i int
		for i, ri.expr = range expr.RHSS {
			ri.invokeExpr()
			if ri.err != nil {
				return
			}
			if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
				ri.rv = ri.rv.Elem()
			}
			if i < len(expr.LHSS) {
				ri.expr = expr.LHSS[i]
				ri.invokeLetExpr()
				if ri.err != nil {
					return
				}
			}

		}

	// TernaryOpExpr
	case *ast.TernaryOpExpr:
		ri.expr = expr.Expr
		ri.invokeExpr()
		if ri.err != nil {
			return
		}

		if toBool(ri.rv) {
			ri.expr = expr.LHS
		} else {
			ri.expr = expr.RHS
		}
		ri.invokeExpr()

	// NilCoalescingOpExpr
	case *ast.NilCoalescingOpExpr:
		// if left side has no error and is not nil, returns left side
		// otherwise returns right side
		ri.expr = expr.LHS
		ri.invokeExpr()
		if ri.err == nil {
			if !isNil(ri.rv) {
				return
			}
		} else {
			ri.err = nil
		}
		ri.expr = expr.RHS
		ri.invokeExpr()

	// LenExpr
	case *ast.LenExpr:
		ri.expr = expr.Expr
		ri.invokeExpr()
		if ri.err != nil {
			return
		}

		if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
			ri.rv = ri.rv.Elem()
		}

		switch ri.rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map, reflect.String, reflect.Chan:
			ri.rv = reflect.ValueOf(int64(ri.rv.Len()))
		default:
			errStr := fmt.Sprintf("type %s does not support len operation", ri.rv.Kind())
			ri.err = newStringError(expr, errStr)
			ri.rv = nilValue
		}

	// ImportExpr
	case *ast.ImportExpr:
		ri.expr = expr.Name
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		ri.rv, ri.err = convertReflectValueToType(ri.rv, stringType)
		if ri.err != nil {
			ri.rv = nilValue
			return
		}
		name := ri.rv.String()
		ri.rv = nilValue

		methods, ok := env.Packages[name]
		if !ok {
			ri.err = newStringError(expr, "package not found: "+name)
			return
		}
		var err error
		pack := ri.env.NewEnv()
		for methodName, methodValue := range methods {
			err = pack.DefineValue(methodName, methodValue)
			if err != nil {
				ri.err = newStringError(expr, "import DefineValue error: "+err.Error())
				return
			}
		}

		types, ok := env.PackageTypes[name]
		if ok {
			for typeName, typeValue := range types {
				err = pack.DefineReflectType(typeName, typeValue)
				if err != nil {
					ri.err = newStringError(expr, "import DefineReflectType error: "+err.Error())
					return
				}
			}
		}

		ri.rv = reflect.ValueOf(pack)

	// MakeExpr
	case *ast.MakeExpr:
		t := makeType(ri, expr.TypeData)
		if ri.err != nil {
			ri.rv = nilValue
			return
		}
		if t == nil {
			ri.err = newStringError(expr, "cannot make type nil")
			ri.rv = nilValue
			return
		}

		switch expr.TypeData.Kind {
		case ast.TypeSlice:
			aLen := 0
			if expr.LenExpr != nil {
				ri.expr = expr.LenExpr
				ri.invokeExpr()
				if ri.err != nil {
					return
				}
				aLen = toInt(ri.rv)
			}
			aCap := aLen
			if expr.CapExpr != nil {
				ri.expr = expr.CapExpr
				ri.invokeExpr()
				if ri.err != nil {
					return
				}
				aCap = toInt(ri.rv)
			}
			if aLen > aCap {
				ri.err = newStringError(expr, "make slice len > cap")
				ri.rv = nilValue
				return
			}
			ri.rv = reflect.MakeSlice(t, aLen, aCap)
			return
		case ast.TypeChan:
			aLen := 0
			if expr.LenExpr != nil {
				ri.expr = expr.LenExpr
				ri.invokeExpr()
				if ri.err != nil {
					return
				}
				aLen = toInt(ri.rv)
			}
			ri.rv = reflect.MakeChan(t, aLen)
			return
		}

		ri.rv, ri.err = makeValue(t)

	// MakeTypeExpr
	case *ast.MakeTypeExpr:
		ri.expr = expr.Type
		ri.invokeExpr()
		if ri.err != nil {
			return
		}

		// if expr.Name has a dot in it, it should give a syntax error, so no needs to check err
		_ = ri.env.DefineReflectType(expr.Name, ri.rv.Type())

		ri.rv = reflect.ValueOf(ri.rv.Type())

	// ChanExpr
	case *ast.ChanExpr:
		ri.expr = expr.RHS
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
			ri.rv = ri.rv.Elem()
		}

		var lhs reflect.Value
		rhs := ri.rv

		if expr.LHS == nil {
			// lhs is nil
			if rhs.Kind() != reflect.Chan {
				// rhs is not channel
				ri.err = newStringError(expr, "receive from non-chan type "+rhs.Kind().String())
				ri.rv = nilValue
				return
			}
		} else {
			// lhs is not nil
			ri.expr = expr.LHS
			ri.invokeExpr()
			if ri.err != nil {
				return
			}
			if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
				ri.rv = ri.rv.Elem()
			}
			if ri.rv.Kind() != reflect.Chan {
				// lhs is not channel
				// lhs <- chan rhs or lhs <- rhs
				ri.err = newStringError(expr, "send to non-chan type "+ri.rv.Kind().String())
				ri.rv = nilValue
				return
			}
			lhs = ri.rv
		}

		var chosen int
		var ok bool

		if rhs.Kind() == reflect.Chan {
			// rhs is channel
			// receive from rhs channel
			cases := []reflect.SelectCase{{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(ri.ctx.Done()),
			}, {
				Dir:  reflect.SelectRecv,
				Chan: rhs,
			}}
			chosen, ri.rv, ok = reflect.Select(cases)
			if chosen == 0 {
				ri.err = ErrInterrupt
				ri.rv = nilValue
				return
			}
			if !ok {
				ri.rv = nilValue
				return
			}
			rhs = ri.rv
		}

		if expr.LHS == nil {
			// <- chan rhs is receive
			return
		}

		// chan lhs <- chan rhs is receive & send
		// or
		// chan lhs <- rhs is send

		ri.rv = nilValue
		rhs, ri.err = convertReflectValueToType(rhs, lhs.Type().Elem())
		if ri.err != nil {
			const format = "cannot use type %s as type %s to send to chan"
			errStr := fmt.Sprintf(format, rhs.Type(), lhs.Type().Elem())
			ri.err = newStringError(expr, errStr)
			return
		}
		// send rhs to lhs channel
		cases := []reflect.SelectCase{{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ri.ctx.Done()),
		}, {
			Dir:  reflect.SelectSend,
			Chan: lhs,
			Send: rhs,
		}}
		if !ri.opts.Debug {
			// captures panic
			defer recoverFunc(ri)
		}
		chosen, _, _ = reflect.Select(cases)
		if chosen == 0 {
			ri.err = ErrInterrupt
		}

	// FuncExpr
	case *ast.FuncExpr:
		ri.expr = expr
		ri.funcExpr()

	// AnonCallExpr
	case *ast.AnonCallExpr:
		ri.expr = expr
		ri.anonCallExpr()

	// CallExpr
	case *ast.CallExpr:
		ri.expr = expr
		ri.callExpr()

	// IncludeExpr
	case *ast.IncludeExpr:
		ri.expr = expr.ItemExpr
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		itemExpr := ri.rv

		ri.expr = expr.ListExpr
		ri.invokeExpr()
		if ri.err != nil {
			return
		}

		if ri.rv.Kind() != reflect.Slice && ri.rv.Kind() != reflect.Array {
			const errStr = "second argument must be slice or array; but have "
			ri.err = newStringError(expr, errStr+ri.rv.Kind().String())
			ri.rv = nilValue
			return
		}

		for i := 0; i < ri.rv.Len(); i++ {
			if equal(itemExpr, ri.rv.Index(i)) {
				ri.rv = trueValue
				return
			}
		}
		ri.rv = falseValue

	default:
		ri.err = newStringError(expr, "unknown expression")
		ri.rv = nilValue
	}
}
