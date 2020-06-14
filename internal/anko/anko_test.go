package anko

import (
	"context"
	"strings"
	"testing"

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

func TestRunContext(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("ok", func(t *testing.T) {
		const s = `
a = 10
println(a)
`
		src := strings.Repeat(s, 1)

		env := NewEnv()
		stmt, err := ParseSrc(src)
		require.NoError(t, err)
		require.NotEqual(t, s, src)

		val, err := RunContext(context.Background(), env, stmt)
		require.NoError(t, err)

		t.Log(val)

		testsuite.IsDestroyed(t, env)
		testsuite.IsDestroyed(t, stmt)
	})

	t.Run("twice", func(t *testing.T) {

	})

}
