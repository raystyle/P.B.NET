package vm

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"project/external/anko/ast"
	"project/external/anko/env"
)

var (
	// ErrBreak when there is an unexpected break statement
	ErrBreak = errors.New("unexpected break statement")
	// ErrContinue when there is an unexpected continue statement
	ErrContinue = errors.New("unexpected continue statement")
	// ErrReturn when there is an unexpected return statement
	ErrReturn = errors.New("unexpected return statement")
	// ErrInterrupt when execution has been interrupted
	ErrInterrupt = errors.New("execution interrupted")
)

// runInfo provides run incoming and outgoing information.
type runInfo struct {
	// incoming
	ctx  context.Context
	env  *env.Env
	opts *Options
	stmt ast.Stmt
	expr ast.Expr
	op   ast.Operator

	// outgoing
	rv  reflect.Value
	err error
}

// runSingleStmt executes statement in the specified environment with context.
// nolint: gocyclo
//gocyclo:ignore
func (ri *runInfo) runSingleStmt() {
	select {
	case <-ri.ctx.Done():
		ri.rv = nilValue
		ri.err = ErrInterrupt
		return
	default:
	}

	switch stmt := ri.stmt.(type) {

	// nil
	case nil:

	// StmtsStmt
	case *ast.StmtsStmt:
		for _, stmt := range stmt.Stmts {
			switch stmt.(type) {
			case *ast.BreakStmt:
				ri.err = ErrBreak
				return
			case *ast.ContinueStmt:
				ri.err = ErrContinue
				return
			case *ast.ReturnStmt:
				ri.stmt = stmt
				ri.runSingleStmt()
				if ri.err != nil {
					return
				}
				ri.err = ErrReturn
				return
			default:
				ri.stmt = stmt
				ri.runSingleStmt()
				if ri.err != nil {
					return
				}
			}
		}

	// ExprStmt
	case *ast.ExprStmt:
		ri.expr = stmt.Expr
		ri.invokeExpr()

	// VarStmt
	case *ast.VarStmt:
		// get right side expression values
		rvs := make([]reflect.Value, len(stmt.Exprs))
		var i int
		for i, ri.expr = range stmt.Exprs {
			ri.invokeExpr()
			if ri.err != nil {
				return
			}
			if e, ok := ri.rv.Interface().(*env.Env); ok {
				rvs[i] = reflect.ValueOf(e.DeepCopy())
			} else {
				rvs[i] = ri.rv
			}
		}

		if len(rvs) == 1 && len(stmt.Names) > 1 {
			// only one right side value but many left side names
			value := rvs[0]
			if value.Kind() == reflect.Interface && !value.IsNil() {
				value = value.Elem()
			}
			if (value.Kind() == reflect.Slice || value.Kind() == reflect.Array) && value.Len() > 0 {
				// value is slice/array, add each value to left side names
				for i := 0; i < value.Len() && i < len(stmt.Names); i++ {
					_ = ri.env.DefineValue(stmt.Names[i], value.Index(i))
				}
				// return last value of slice/array
				ri.rv = value.Index(value.Len() - 1)
				return
			}
		}

		// define all names with right side values
		for i = 0; i < len(rvs) && i < len(stmt.Names); i++ {
			_ = ri.env.DefineValue(stmt.Names[i], rvs[i])
		}

		// return last right side value
		ri.rv = rvs[len(rvs)-1]

	// LetsStmt
	case *ast.LetsStmt:
		// get right side expression values
		rvs := make([]reflect.Value, len(stmt.RHSS))
		var i int
		for i, ri.expr = range stmt.RHSS {
			ri.invokeExpr()
			if ri.err != nil {
				return
			}
			if e, ok := ri.rv.Interface().(*env.Env); ok {
				rvs[i] = reflect.ValueOf(e.DeepCopy())
			} else {
				rvs[i] = ri.rv
			}
		}

		if len(rvs) == 1 && len(stmt.LHSS) > 1 {
			// only one right side value but many left side expressions
			value := rvs[0]
			if value.Kind() == reflect.Interface && !value.IsNil() {
				value = value.Elem()
			}
			if (value.Kind() == reflect.Slice || value.Kind() == reflect.Array) && value.Len() > 0 {
				// value is slice/array, add each value to left side expression
				for i := 0; i < value.Len() && i < len(stmt.LHSS); i++ {
					ri.rv = value.Index(i)
					ri.expr = stmt.LHSS[i]
					ri.invokeLetExpr()
					if ri.err != nil {
						return
					}
				}
				// return last value of slice/array
				ri.rv = value.Index(value.Len() - 1)
				return
			}
		}

		// invoke all left side expressions with right side values
		for i = 0; i < len(rvs) && i < len(stmt.LHSS); i++ {
			value := rvs[i]
			if value.Kind() == reflect.Interface && !value.IsNil() {
				value = value.Elem()
			}
			ri.rv = value
			ri.expr = stmt.LHSS[i]
			ri.invokeLetExpr()
			if ri.err != nil {
				return
			}
		}

		// return last right side value
		ri.rv = rvs[len(rvs)-1]

	// LetMapItemStmt
	case *ast.LetMapItemStmt:
		ri.expr = stmt.RHS
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		var rvs []reflect.Value
		if isNil(ri.rv) {
			rvs = []reflect.Value{nilValue, falseValue}
		} else {
			rvs = []reflect.Value{ri.rv, trueValue}
		}
		var i int
		for i, ri.expr = range stmt.LHSS {
			ri.rv = rvs[i]
			if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
				ri.rv = ri.rv.Elem()
			}
			ri.invokeLetExpr()
			if ri.err != nil {
				return
			}
		}
		ri.rv = rvs[0]

	// IfStmt
	case *ast.IfStmt:
		// if
		ri.expr = stmt.If
		ri.invokeExpr()
		if ri.err != nil {
			return
		}

		e := ri.env

		if toBool(ri.rv) {
			// then
			ri.rv = nilValue
			ri.stmt = stmt.Then
			ri.env = e.NewEnv()
			ri.runSingleStmt()
			ri.env = e
			return
		}

		for _, statement := range stmt.ElseIf {
			elseIf := statement.(*ast.IfStmt)

			// else if - if
			ri.env = e.NewEnv()
			ri.expr = elseIf.If
			ri.invokeExpr()
			if ri.err != nil {
				ri.env = e
				return
			}

			if !toBool(ri.rv) {
				continue
			}

			// else if - then
			ri.rv = nilValue
			ri.stmt = elseIf.Then
			ri.env = e.NewEnv()
			ri.runSingleStmt()
			ri.env = e
			return
		}

		if stmt.Else != nil {
			// else
			ri.rv = nilValue
			ri.stmt = stmt.Else
			ri.env = e.NewEnv()
			ri.runSingleStmt()
		}

		ri.env = e

	// TryStmt
	case *ast.TryStmt:
		// only the try statement will ignore any error except ErrInterrupt
		// all other parts will return the error

		e := ri.env
		ri.env = e.NewEnv()

		ri.stmt = stmt.Try
		ri.runSingleStmt()

		if ri.err != nil {
			if ri.err == ErrInterrupt {
				ri.env = e
				return
			}

			// Catch
			ri.stmt = stmt.Catch
			if stmt.Var != "" {
				_ = ri.env.DefineValue(stmt.Var, reflect.ValueOf(ri.err))
			}
			ri.err = nil
			ri.runSingleStmt()
			if ri.err != nil {
				ri.env = e
				return
			}
		}

		if stmt.Finally != nil {
			// Finally
			ri.stmt = stmt.Finally
			ri.runSingleStmt()
		}

		ri.env = e

	// LoopStmt
	case *ast.LoopStmt:
		e := ri.env
		ri.env = e.NewEnv()

		for {
			select {
			case <-ri.ctx.Done():
				ri.err = ErrInterrupt
				ri.rv = nilValue
				ri.env = e
				return
			default:
			}

			if stmt.Expr != nil {
				ri.expr = stmt.Expr
				ri.invokeExpr()
				if ri.err != nil {
					break
				}
				if !toBool(ri.rv) {
					break
				}
			}

			ri.stmt = stmt.Stmt
			ri.runSingleStmt()
			if ri.err != nil {
				if ri.err == ErrContinue {
					ri.err = nil
					continue
				}
				if ri.err == ErrReturn {
					ri.env = e
					return
				}
				if ri.err == ErrBreak {
					ri.err = nil
				}
				break
			}
		}

		ri.rv = nilValue
		ri.env = e

	// ForStmt
	case *ast.ForStmt:
		ri.expr = stmt.Value
		ri.invokeExpr()
		value := ri.rv
		if ri.err != nil {
			return
		}
		if value.Kind() == reflect.Interface && !value.IsNil() {
			value = value.Elem()
		}

		e := ri.env
		ri.env = e.NewEnv()

		switch value.Kind() {
		case reflect.Slice, reflect.Array:
			for i := 0; i < value.Len(); i++ {
				select {
				case <-ri.ctx.Done():
					ri.err = ErrInterrupt
					ri.rv = nilValue
					ri.env = e
					return
				default:
				}

				iv := value.Index(i)
				if iv.Kind() == reflect.Interface && !iv.IsNil() {
					iv = iv.Elem()
				}
				if iv.Kind() == reflect.Ptr {
					iv = iv.Elem()
				}
				_ = ri.env.DefineValue(stmt.Vars[0], iv)

				ri.stmt = stmt.Stmt
				ri.runSingleStmt()
				if ri.err != nil {
					if ri.err == ErrContinue {
						ri.err = nil
						continue
					}
					if ri.err == ErrReturn {
						ri.env = e
						return
					}
					if ri.err == ErrBreak {
						ri.err = nil
					}
					break
				}
			}
			ri.rv = nilValue
			ri.env = e

		case reflect.Map:
			keys := value.MapKeys()
			for i := 0; i < len(keys); i++ {
				select {
				case <-ri.ctx.Done():
					ri.err = ErrInterrupt
					ri.rv = nilValue
					ri.env = e
					return
				default:
				}

				_ = ri.env.DefineValue(stmt.Vars[0], keys[i])

				if len(stmt.Vars) > 1 {
					_ = ri.env.DefineValue(stmt.Vars[1], value.MapIndex(keys[i]))
				}

				ri.stmt = stmt.Stmt
				ri.runSingleStmt()
				if ri.err != nil {
					if ri.err == ErrContinue {
						ri.err = nil
						continue
					}
					if ri.err == ErrReturn {
						ri.env = e
						return
					}
					if ri.err == ErrBreak {
						ri.err = nil
					}
					break
				}
			}
			ri.rv = nilValue
			ri.env = e

		case reflect.Chan:
			var chosen int
			var ok bool
			for {
				cases := []reflect.SelectCase{{
					Dir:  reflect.SelectRecv,
					Chan: reflect.ValueOf(ri.ctx.Done()),
				}, {
					Dir:  reflect.SelectRecv,
					Chan: value,
				}}
				chosen, ri.rv, ok = reflect.Select(cases)
				if chosen == 0 {
					ri.err = ErrInterrupt
					ri.rv = nilValue
					break
				}
				if !ok {
					break
				}

				if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
					ri.rv = ri.rv.Elem()
				}
				if ri.rv.Kind() == reflect.Ptr {
					ri.rv = ri.rv.Elem()
				}

				_ = ri.env.DefineValue(stmt.Vars[0], ri.rv)

				ri.stmt = stmt.Stmt
				ri.runSingleStmt()
				if ri.err != nil {
					if ri.err == ErrContinue {
						ri.err = nil
						continue
					}
					if ri.err == ErrReturn {
						ri.env = e
						return
					}
					if ri.err == ErrBreak {
						ri.err = nil
					}
					break
				}
			}
			ri.rv = nilValue
			ri.env = e

		default:
			errStr := "for cannot loop over type " + value.Kind().String()
			ri.err = newStringError(stmt, errStr)
			ri.rv = nilValue
			ri.env = e
		}

	// CForStmt
	case *ast.CForStmt:
		e := ri.env
		ri.env = e.NewEnv()

		if stmt.Stmt1 != nil {
			ri.stmt = stmt.Stmt1
			ri.runSingleStmt()
			if ri.err != nil {
				ri.env = e
				return
			}
		}

		for {
			select {
			case <-ri.ctx.Done():
				ri.err = ErrInterrupt
				ri.rv = nilValue
				ri.env = e
				return
			default:
			}

			if stmt.Expr2 != nil {
				ri.expr = stmt.Expr2
				ri.invokeExpr()
				if ri.err != nil {
					break
				}
				if !toBool(ri.rv) {
					break
				}
			}

			ri.stmt = stmt.Stmt
			ri.runSingleStmt()
			if ri.err == ErrContinue {
				ri.err = nil
			}
			if ri.err != nil {
				if ri.err == ErrReturn {
					ri.env = e
					return
				}
				if ri.err == ErrBreak {
					ri.err = nil
				}
				break
			}

			if stmt.Expr3 != nil {
				ri.expr = stmt.Expr3
				ri.invokeExpr()
				if ri.err != nil {
					break
				}
			}
		}
		ri.rv = nilValue
		ri.env = e

	// ReturnStmt
	case *ast.ReturnStmt:
		switch len(stmt.Exprs) {
		case 0:
			ri.rv = nilValue
			return
		case 1:
			ri.expr = stmt.Exprs[0]
			ri.invokeExpr()
			return
		}
		rvs := make([]interface{}, len(stmt.Exprs))
		var i int
		for i, ri.expr = range stmt.Exprs {
			ri.invokeExpr()
			if ri.err != nil {
				return
			}
			rvs[i] = ri.rv.Interface()
		}
		ri.rv = reflect.ValueOf(rvs)

	// ThrowStmt
	case *ast.ThrowStmt:
		ri.expr = stmt.Expr
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		ri.err = newStringError(stmt, fmt.Sprint(ri.rv.Interface()))

	// ModuleStmt
	case *ast.ModuleStmt:
		e := ri.env
		ri.env, ri.err = e.NewModule(stmt.Name)
		if ri.err != nil {
			return
		}
		ri.stmt = stmt.Stmt
		ri.runSingleStmt()
		ri.env = e
		if ri.err != nil {
			return
		}
		ri.rv = nilValue

	// SelectStmt
	case *ast.SelectStmt:
		e := ri.env
		ri.env = e.NewEnv()
		body := stmt.Body.(*ast.SelectBodyStmt)
		letsExprs := []ast.Expr{nil}
		bodies := []ast.Stmt{nil}
		cases := []reflect.SelectCase{{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ri.ctx.Done()),
			Send: zeroValue,
		}}
		for _, selectCaseStmt := range body.Cases {
			caseStmt := selectCaseStmt.(*ast.SelectCaseStmt)
			var letExpr ast.Expr
			var che *ast.ChanExpr
			var ok bool
			pos := caseStmt.Expr
			switch e := caseStmt.Expr.(type) {
			case *ast.LetsStmt:
				letExpr = e.LHSS[0]
				pos = e.RHSS[0]
				che, ok = e.RHSS[0].(*ast.ChanExpr)
			case *ast.ExprStmt: // "case <-a:", "case a <- 1:"(first)
				pos = e.Expr
				che, ok = e.Expr.(*ast.ChanExpr)
			case *ast.ChanStmt: // "case v = <-a:", "case v, ok = <-a:"
				letExpr = e.LHS
				pos = e.RHS
				che = &ast.ChanExpr{RHS: e.RHS}
				ok = true
			}
			if !ok {
				ri.err = newStringError(pos, "invalid operation")
				ri.rv = nilValue
				return
			}
			// send operation
			if che.LHS != nil {
				// get channel in left
				ri.expr = che.LHS
				ri.invokeExpr()
				if ri.err != nil {
					return
				}
				kind := ri.rv.Kind()
				if kind != reflect.Chan {
					ri.err = newStringError(pos, "can't send to "+kind.String())
					ri.rv = nilValue
					return
				}
				ch := ri.rv
				// get value in right that will be send
				ri.expr = che.RHS
				ri.invokeExpr()
				if ri.err != nil {
					return
				}
				letsExprs = append(letsExprs, nil)
				bodies = append(bodies, caseStmt.Stmt)
				cases = append(cases, reflect.SelectCase{
					Dir:  reflect.SelectSend,
					Chan: ch,
					Send: ri.rv,
				})
			} else {
				ri.expr = che.RHS
				ri.invokeExpr()
				if ri.err != nil {
					return
				}
				kind := ri.rv.Kind()
				if kind != reflect.Chan {
					ri.err = newStringError(pos, "can't receive from "+kind.String())
					ri.rv = nilValue
					return
				}
				letsExprs = append(letsExprs, letExpr)
				bodies = append(bodies, caseStmt.Stmt)
				cases = append(cases, reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: ri.rv,
					Send: zeroValue,
				})
			}
		}
		if body.Default != nil {
			letsExprs = append(letsExprs, nil)
			bodies = append(bodies, body.Default)
			cases = append(cases, reflect.SelectCase{
				Dir:  reflect.SelectDefault,
				Chan: zeroValue,
				Send: zeroValue,
			})
		}
		if !ri.opts.Debug {
			// captures panic
			defer recoverFunc(ri)
		}
		chosen, rv, _ := reflect.Select(cases)
		if chosen == 0 {
			ri.err = ErrInterrupt
			ri.rv = nilValue
			ri.env = e
			return
		}
		if letExpr := letsExprs[chosen]; letExpr != nil {
			ri.expr = letExpr
			ri.rv = rv
			ri.invokeLetExpr()
			if ri.err != nil {
				return
			}
		}
		if statement := bodies[chosen]; statement != nil {
			if tmp, ok := statement.(*ast.SelectCaseStmt); ok && tmp.Stmt == nil {
				ri.env = e
				return
			}
			ri.stmt = statement
			ri.runSingleStmt()
		}
		ri.env = e
		return

	// SwitchStmt
	case *ast.SwitchStmt:
		e := ri.env
		ri.env = e.NewEnv()

		ri.expr = stmt.Expr
		ri.invokeExpr()
		if ri.err != nil {
			ri.env = e
			return
		}
		value := ri.rv

		for _, switchCaseStmt := range stmt.Cases {
			caseStmt := switchCaseStmt.(*ast.SwitchCaseStmt)
			for _, ri.expr = range caseStmt.Exprs {
				ri.invokeExpr()
				if ri.err != nil {
					ri.env = e
					return
				}
				if equal(ri.rv, value) {
					ri.stmt = caseStmt.Stmt
					ri.runSingleStmt()
					ri.env = e
					return
				}
			}
		}

		if stmt.Default == nil {
			ri.rv = nilValue
		} else {
			ri.stmt = stmt.Default
			ri.runSingleStmt()
		}

		ri.env = e

	// GoroutineStmt
	case *ast.GoroutineStmt:
		ri.expr = stmt.Expr
		ri.invokeExpr()

	// DeleteStmt
	case *ast.DeleteStmt:
		ri.expr = stmt.Item
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		item := ri.rv

		if stmt.Key != nil {
			ri.expr = stmt.Key
			ri.invokeExpr()
			if ri.err != nil {
				return
			}
		}

		if item.Kind() == reflect.Interface && !item.IsNil() {
			item = item.Elem()
		}

		switch item.Kind() {
		case reflect.String:
			if stmt.Key != nil && ri.rv.Kind() == reflect.Bool && ri.rv.Bool() {
				ri.env.DeleteGlobal(item.String())
				ri.rv = nilValue
				return
			}
			ri.env.Delete(item.String())
			ri.rv = nilValue

		case reflect.Map:
			if stmt.Key == nil {
				const errStr = "second argument to delete cannot be nil for map"
				ri.err = newStringError(stmt, errStr)
				ri.rv = nilValue
				return
			}
			if item.IsNil() {
				ri.rv = nilValue
				return
			}
			ri.rv, ri.err = convertReflectValueToType(ri.rv, item.Type().Key())
			if ri.err != nil {
				const format = "cannot use type %s as type %s in delete"
				errStr := fmt.Sprintf(format, item.Type().Key(), ri.rv.Type())
				ri.err = newStringError(stmt, errStr)
				ri.rv = nilValue
				return
			}
			item.SetMapIndex(ri.rv, reflect.Value{})
			ri.rv = nilValue
		default:
			errStr := "first argument to delete cannot be type " + item.Kind().String()
			ri.err = newStringError(stmt, errStr)
			ri.rv = nilValue
		}

	// CloseStmt
	case *ast.CloseStmt:
		ri.expr = stmt.Expr
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		if ri.rv.Kind() == reflect.Chan {
			ri.rv.Close()
			ri.rv = nilValue
			return
		}
		errStr := "type cannot be " + ri.rv.Kind().String() + " for close"
		ri.err = newStringError(stmt, errStr)
		ri.rv = nilValue

	// ChanStmt
	case *ast.ChanStmt:
		ri.expr = stmt.RHS
		ri.invokeExpr()
		if ri.err != nil {
			return
		}
		if ri.rv.Kind() == reflect.Interface && !ri.rv.IsNil() {
			ri.rv = ri.rv.Elem()
		}

		if ri.rv.Kind() != reflect.Chan {
			// rhs is not channel
			errStr := "receive from non-chan type " + ri.rv.Kind().String()
			ri.err = newStringError(stmt, errStr)
			ri.rv = nilValue
			return
		}

		// rhs is channel
		// receive from rhs channel
		rhs := ri.rv
		cases := []reflect.SelectCase{{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ri.ctx.Done()),
		}, {
			Dir:  reflect.SelectRecv,
			Chan: rhs,
		}}
		var chosen int
		var ok bool
		chosen, ri.rv, ok = reflect.Select(cases)
		if chosen == 0 {
			ri.err = ErrInterrupt
			ri.rv = nilValue
			return
		}

		rhs = ri.rv // store rv in rhs temporarily

		if stmt.OkExpr != nil {
			// set ok to OkExpr
			if ok {
				ri.rv = trueValue
			} else {
				ri.rv = falseValue
			}
			ri.expr = stmt.OkExpr
			ri.invokeLetExpr()
		}

		if ok {
			// set rv to lhs
			ri.rv = rhs
			ri.expr = stmt.LHS
			ri.invokeLetExpr()
			if ri.err != nil {
				return
			}
		} else {
			ri.rv = nilValue
		}

	default:
		ri.err = newStringError(stmt, "unknown statement")
		ri.rv = nilValue
	}
}
