package vm

import (
	"reflect"
	"strings"

	"project/external/anko/ast"
)

// nolint: gocyclo
//gocyclo:ignore
func (ri *runInfo) invokeOperator() {
	switch op := ri.op.(type) {

	// BinaryOperator
	case *ast.BinaryOperator:
		ri.expr = op.LHS
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
			ri.rv = ri.rv.Elem()
		}

		switch op.Operator {
		case "||":
			if toBool(ri.rv) {
				ri.rv = trueValue
				return
			}
		case "&&":
			if !toBool(ri.rv) {
				ri.rv = falseValue
				return
			}
		default:
			ri.err = newStringError(op, "unknown operator")
			ri.rv = nilValue
			return
		}

		ri.expr = op.RHS
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
			ri.rv = ri.rv.Elem()
		}

		if toBool(ri.rv) {
			ri.rv = trueValue
		} else {
			ri.rv = falseValue
		}

	// ComparisonOperator
	case *ast.ComparisonOperator:
		ri.expr = op.LHS
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
			ri.rv = ri.rv.Elem()
		}
		lhsV := ri.rv

		ri.expr = op.RHS
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
			ri.rv = ri.rv.Elem()
		}

		switch op.Operator {
		case "==":
			ri.rv = reflect.ValueOf(equal(lhsV, ri.rv))
		case "!=":
			ri.rv = reflect.ValueOf(!equal(lhsV, ri.rv))
		case "<":
			ri.rv = reflect.ValueOf(toFloat64(lhsV) < toFloat64(ri.rv))
		case "<=":
			ri.rv = reflect.ValueOf(toFloat64(lhsV) <= toFloat64(ri.rv))
		case ">":
			ri.rv = reflect.ValueOf(toFloat64(lhsV) > toFloat64(ri.rv))
		case ">=":
			ri.rv = reflect.ValueOf(toFloat64(lhsV) >= toFloat64(ri.rv))
		default:
			ri.err = newStringError(op, "unknown operator")
			ri.rv = nilValue
		}

	// AddOperator
	case *ast.AddOperator:
		ri.expr = op.LHS
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
			ri.rv = ri.rv.Elem()
		}
		lhsV := ri.rv

		ri.expr = op.RHS
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
			ri.rv = ri.rv.Elem()
		}

		switch op.Operator {
		case "+":
			lhsKind := lhsV.Kind()
			rhsKind := ri.rv.Kind()

			if lhsKind == reflect.Slice || lhsKind == reflect.Array {
				if rhsKind == reflect.Slice || rhsKind == reflect.Array {
					// append slice to slice
					ri.rv, ri.err = appendSlice(op, lhsV, ri.rv)
					return
				}
				// try to append rhs non-slice to lhs slice
				ri.rv, ri.err = convertReflectValueToType(ri.rv, lhsV.Type().Elem())
				if ri.err != nil {
					ri.err = newStringError(op, "invalid type conversion")
					ri.rv = nilValue
					return
				}
				ri.rv = reflect.Append(lhsV, ri.rv)
				return
			}
			if rhsKind == reflect.Slice || rhsKind == reflect.Array {
				// can not append rhs slice to lhs non-slice
				ri.err = newStringError(op, "invalid type conversion")
				ri.rv = nilValue
				return
			}

			kind := precedenceOfKinds(lhsKind, rhsKind)
			switch kind {
			case reflect.String:
				ri.rv = reflect.ValueOf(toString(lhsV) + toString(ri.rv))
			case reflect.Float64, reflect.Float32:
				ri.rv = reflect.ValueOf(toFloat64(lhsV) + toFloat64(ri.rv))
			default:
				ri.rv = reflect.ValueOf(toInt64(lhsV) + toInt64(ri.rv))
			}

		case "-":
			switch lhsV.Kind() {
			case reflect.Float64, reflect.Float32:
				ri.rv = reflect.ValueOf(toFloat64(lhsV) - toFloat64(ri.rv))
				return
			}
			switch ri.rv.Kind() {
			case reflect.Float64, reflect.Float32:
				ri.rv = reflect.ValueOf(toFloat64(lhsV) - toFloat64(ri.rv))
			default:
				ri.rv = reflect.ValueOf(toInt64(lhsV) - toInt64(ri.rv))
			}

		case "|":
			ri.rv = reflect.ValueOf(toInt64(lhsV) | toInt64(ri.rv))
		default:
			ri.err = newStringError(op, "unknown operator")
			ri.rv = nilValue
		}

	// MultiplyOperator
	case *ast.MultiplyOperator:
		ri.expr = op.LHS
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
			ri.rv = ri.rv.Elem()
		}
		lhsV := ri.rv

		ri.expr = op.RHS
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
			ri.rv = ri.rv.Elem()
		}

		switch op.Operator {
		case "*":
			if lhsV.Kind() == reflect.String && (ri.rv.Kind() == reflect.Int ||
				ri.rv.Kind() == reflect.Int32 || ri.rv.Kind() == reflect.Int64) {
				ri.rv = reflect.ValueOf(strings.Repeat(toString(lhsV), int(toInt64(ri.rv))))
				return
			}
			if lhsV.Kind() == reflect.Float64 || ri.rv.Kind() == reflect.Float64 {
				ri.rv = reflect.ValueOf(toFloat64(lhsV) * toFloat64(ri.rv))
				return
			}
			ri.rv = reflect.ValueOf(toInt64(lhsV) * toInt64(ri.rv))
		case "/":
			ri.rv = reflect.ValueOf(toFloat64(lhsV) / toFloat64(ri.rv))
		case "%":
			ri.rv = reflect.ValueOf(toInt64(lhsV) % toInt64(ri.rv))
		case ">>":
			ri.rv = reflect.ValueOf(toInt64(lhsV) >> uint64(toInt64(ri.rv)))
		case "<<":
			ri.rv = reflect.ValueOf(toInt64(lhsV) << uint64(toInt64(ri.rv)))
		case "&":
			ri.rv = reflect.ValueOf(toInt64(lhsV) & toInt64(ri.rv))
		default:
			ri.err = newStringError(op, "unknown operator")
			ri.rv = nilValue
		}

	default:
		ri.err = newStringError(op, "unknown operator")
		ri.rv = nilValue
	}
}
