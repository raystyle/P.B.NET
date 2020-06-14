package anko

import (
	"context"
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
		val, err := RunContext(context.Background(), env, stmt)
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
		ctx := context.Background()

		env1 := NewEnv()
		val, err := RunContext(ctx, env1, stmt)
		require.NoError(t, err)
		t.Log(val)
		testsuite.IsDestroyed(t, env1)

		env2 := NewEnv()
		val, err = RunContext(ctx, env2, stmt)
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
		val, err := RunContext(context.Background(), env, stmt)
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

func testRun(t *testing.T, s string) {
	stmt := testParseSrc(t, s)

	env := NewEnv()
	val, err := RunContext(context.Background(), env, stmt)
	require.NoError(t, err)

	t.Log(val)

	testsuite.IsDestroyed(t, env)
	testsuite.IsDestroyed(t, stmt)
}

func TestCore(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("keys", func(t *testing.T) {
		const src = `
m = {"foo": "bar", "bar": "baz"}
for key in keys(m) {
	println(key, m[key])
}
`
		testRun(t, src)
	})

	t.Run("range", func(t *testing.T) {

	})

	t.Run("typeOf", func(t *testing.T) {

	})

	t.Run("kindOf", func(t *testing.T) {

	})

	t.Run("eval", func(t *testing.T) {

	})
}
