package anko

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mattn/anko/ast"
	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestNewEnv(t *testing.T) {
	env := NewEnv()
	require.NotNil(t, env)

	fmt.Println(Packages)
	fmt.Println(Types)

	v, err := env.Get("keys")
	require.NoError(t, err)
	require.NotNil(t, v)

	testsuite.IsDestroyed(t, env)
}

func TestParseSrc(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		const s = `
a = 10
println(a)
`
		src := strings.Repeat(s, 1)
		stmt, err := ParseSrc(src)
		require.NoError(t, err)
		require.NotNil(t, stmt)
		require.NotEqual(t, s, src)

		testsuite.IsDestroyed(t, stmt)
	})

	t.Run("invalid", func(t *testing.T) {
		const s = `
a = 10
println(a)
a -- a
`
		src := strings.Repeat(s, 1)
		stmt, err := ParseSrc(src)
		require.Error(t, err)
		require.Nil(t, stmt)
		require.NotEqual(t, s, src)

		t.Log(err)
	})
}

func testParseSrc(t *testing.T, s string) ast.Stmt {
	src := strings.Repeat(s, 1)
	stmt, err := ParseSrc(src)
	require.NoError(t, err)
	require.NotEqual(t, s, src)
	return stmt
}

func TestRunContext(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("ok", func(t *testing.T) {
		const src = `
a = 10
println(a)
`
		stmt := testParseSrc(t, src)

		env := NewEnv()
		val, err := Run(env, stmt)
		require.NoError(t, err)

		t.Log(val)

		testsuite.IsDestroyed(t, env)
		testsuite.IsDestroyed(t, stmt)
	})

	t.Run("run one stmt twice", func(t *testing.T) {
		const src = `
a = 10
println(a)
`
		stmt := testParseSrc(t, src)

		env1 := NewEnv()
		val, err := Run(env1, stmt)
		require.NoError(t, err)
		t.Log(val)
		testsuite.IsDestroyed(t, env1)

		env2 := NewEnv()
		val, err = Run(env2, stmt)
		require.NoError(t, err)
		t.Log(val)
		testsuite.IsDestroyed(t, env2)

		testsuite.IsDestroyed(t, stmt)
	})

	t.Run("run error", func(t *testing.T) {
		const src = `
a = 10
println(a)

println(b)
`
		stmt := testParseSrc(t, src)

		env := NewEnv()
		val, err := Run(env, stmt)
		require.Error(t, err)

		t.Log(val, err)

		testsuite.IsDestroyed(t, env)
		testsuite.IsDestroyed(t, stmt)
	})

	t.Run("cancel", func(t *testing.T) {
		env := NewEnv()
		_ = env.Define("sleep", func() {
			time.Sleep(time.Second)
		})

		const src = `
a = 10
for {
	println(a)
	sleep()
}
`
		stmt := testParseSrc(t, src)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		val, err := RunContext(ctx, env, stmt)
		require.Error(t, err)

		t.Log(val, err)

		testsuite.IsDestroyed(t, env)
		testsuite.IsDestroyed(t, stmt)
	})
}

func testRun(t *testing.T, s string, fail bool) {
	stmt := testParseSrc(t, s)

	env := NewEnv()
	val, err := Run(env, stmt)
	if fail {
		require.Error(t, err)
		t.Log(val, err)
	} else {
		require.NoError(t, err)
		t.Log(val)
	}

	testsuite.IsDestroyed(t, env)
	testsuite.IsDestroyed(t, stmt)
}

func TestCoreKeys(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
m = {"foo": "bar", "bar": "baz"}
for key in keys(m) {
	println(key, m[key])
}
`
	testRun(t, src, false)
}

func TestCoreRange(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("np", func(t *testing.T) {
		const src = `range()`
		testRun(t, src, true)
	})

	t.Run("1p", func(t *testing.T) {
		const src = `range(3)`
		testRun(t, src, false)
	})

	t.Run("2p", func(t *testing.T) {
		const src = `range(1, 3)`
		testRun(t, src, false)
	})

	t.Run("3p", func(t *testing.T) {
		const src = `range(1, 10, 2)`
		testRun(t, src, false)
	})

	t.Run("3p-zero step", func(t *testing.T) {
		const src = `range(1, 10, 0)`
		testRun(t, src, true)
	})

	t.Run("4p", func(t *testing.T) {
		const src = `range(1, 2, 3, 4)`
		testRun(t, src, true)
	})
}

func TestCoreTypeOf(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
a = 10
println(typeOf(a))
`
	testRun(t, src, false)
}

func TestCoreKindOf(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("int64", func(t *testing.T) {
		const src = `
a = 10
println(kindOf(a))
`
		testRun(t, src, false)
	})

	t.Run("nil", func(t *testing.T) {
		const src = `
a = nil
println(kindOf(a))
`
		testRun(t, src, false)
	})
}

func TestCoreEval(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("ok", func(t *testing.T) {
		const src = `eval("println('in eval')")`
		testRun(t, src, false)
	})

	t.Run("invalid source", func(t *testing.T) {
		const src = `eval("a -- a")`
		testRun(t, src, true)
	})

	t.Run("eval with error", func(t *testing.T) {
		const src = "eval(`" + `
a = 10
println(a)

println(b)
` + "`)"
		testRun(t, src, true)
	})
}
