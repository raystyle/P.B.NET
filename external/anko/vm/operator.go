package vm

import (
	"reflect"

	"project/external/anko/ast"
)

func (runInfo *runInfoStruct) invokeOperator() {
	switch operator := runInfo.operator.(type) {

	// BinaryOperator
	case *ast.BinaryOperator:
		runInfo.expr = operator.LHS
		runInfo.invokeExpr()
		if runInfo.err != nil {
			return
		}
		if runInfo.rv.Kind() == reflect.Interface && !runInfo.rv.IsNil() {
			runInfo.rv = runInfo.rv.Elem()
		}

		switch operator.Operator {
		case "||":
			if toBool(runInfo.rv) {
				runInfo.rv = trueValue
				return
			}
		case "&&":
			if !toBool(runInfo.rv) {
				runInfo.rv = falseValue
				return
			}
		default:
			runInfo.err = newStringError(operator, "unknown operator")
			runInfo.rv = nilValue
			return
		}

		runInfo.expr = operator.RHS
		runInfo.invokeExpr()
		if runInfo.err != nil {
			return
		}
		if runInfo.rv.Kind() == reflect.Interface && !runInfo.rv.IsNil() {
			runInfo.rv = runInfo.rv.Elem()
		}

		if toBool(runInfo.rv) {
			runInfo.rv = trueValue
		} else {
			runInfo.rv = falseValue
		}

	// ComparisonOperator
	case *ast.ComparisonOperator:
		runInfo.expr = operator.LHS
		runInfo.invokeExpr()
		if runInfo.err != nil {
			return
		}
		if runInfo.rv.Kind() == reflect.Interface && !runInfo.rv.IsNil() {
			runInfo.rv = runInfo.rv.Elem()
		}
		lhsV := runInfo.rv

		runInfo.expr = operator.RHS
		runInfo.invokeExpr()
		if runInfo.err != nil {
			return
		}
		if runInfo.rv.Kind() == reflect.Interface && !runInfo.rv.IsNil() {
			runInfo.rv = runInfo.rv.Elem()
		}

		switch operator.Operator {
		case "==":
			runInfo.rv = reflect.ValueOf(equal(lhsV, runInfo.rv))
		case "!=":
			runInfo.rv = reflect.ValueOf(!equal(lhsV, runInfo.rv))
		case "<":
			runInfo.rv = reflect.ValueOf(toFloat64(lhsV) < toFloat64(runInfo.rv))
		case "<=":
			runInfo.rv = reflect.ValueOf(toFloat64(lhsV) <= toFloat64(runInfo.rv))
		case ">":
			runInfo.rv = reflect.ValueOf(toFloat64(lhsV) > toFloat64(runInfo.rv))
		case ">=":
			runInfo.rv = reflect.ValueOf(toFloat64(lhsV) >= toFloat64(runInfo.rv))
		default:
			runInfo.err = newStringError(operator, "unknown operator")
			runInfo.rv = nilValue
		}

	default:
		runInfo.err = newStringError(operator, "unknown operator")
		runInfo.rv = nilValue
	}
}
