package vm

import (
	"fmt"
	"reflect"

	"project/external/anko/ast"
)

// invokeExpr evaluates one expression.
func (runInfo *runInfoStruct) invokeExpr() {
	switch expr := runInfo.expr.(type) {

	// OpExpr
	case *ast.OpExpr:
		runInfo.operator = expr.Op
		runInfo.invokeOperator()

	// IdentExpr
	case *ast.IdentExpr:
		runInfo.rv, runInfo.err = runInfo.env.GetValue(expr.Lit)
		if runInfo.err != nil {
			runInfo.err = newError(expr, runInfo.err)
		}

	// LiteralExpr
	case *ast.LiteralExpr:
		runInfo.rv = expr.Literal

	// ArrayExpr
	case *ast.ArrayExpr:
		if expr.TypeData == nil {
			slice := make([]interface{}, len(expr.Exprs))
			var i int
			for i, runInfo.expr = range expr.Exprs {
				runInfo.invokeExpr()
				if runInfo.err != nil {
					return
				}
				slice[i] = runInfo.rv.Interface()
			}
			runInfo.rv = reflect.ValueOf(slice)
			return
		}

		t := makeType(runInfo, expr.TypeData)
		if runInfo.err != nil {
			runInfo.rv = nilValue
			return
		}
		if t == nil {
			runInfo.err = newStringError(expr, "cannot make type nil")
			runInfo.rv = nilValue
			return
		}

		slice := reflect.MakeSlice(t, len(expr.Exprs), len(expr.Exprs))
		var i int
		valueType := t.Elem()
		for i, runInfo.expr = range expr.Exprs {
			runInfo.invokeExpr()
			if runInfo.err != nil {
				return
			}

			runInfo.rv, runInfo.err = convertReflectValueToType(runInfo.rv, valueType)
			if runInfo.err != nil {
				const format = "cannot use type %s as type %s as slice value"
				errStr := fmt.Sprintf(format, runInfo.rv.Type(), valueType)
				runInfo.err = newStringError(expr, errStr)
				runInfo.rv = nilValue
				return
			}

			slice.Index(i).Set(runInfo.rv)
		}
		runInfo.rv = slice

	default:
		runInfo.err = newStringError(expr, "unknown expression")
		runInfo.rv = nilValue
	}
}
