package vm

import (
	"project/external/anko/ast"
)

func (runInfo *runInfoStruct) invokeLetExpr() {
	switch expr := runInfo.expr.(type) {

	// IdentExpr
	case *ast.IdentExpr:
		if runInfo.env.SetValue(expr.Lit, runInfo.rv) != nil {
			runInfo.err = nil
			runInfo.env.DefineValue(expr.Lit, runInfo.rv)
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
