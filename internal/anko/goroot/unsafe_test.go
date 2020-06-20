package goroot

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/anko"
	"project/internal/testsuite"
)

func testRun(t *testing.T, s string, fail bool) {
	src := strings.Repeat(s, 1)
	stmt, err := anko.ParseSrc(src)
	require.NoError(t, err)
	require.NotEqual(t, s, src)

	env := anko.NewEnv()
	val, err := anko.Run(env, stmt)
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

func TestUnsafe(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
unsafe = import("unsafe")

Float64 = 0.614
Int64 = 123

println(unsafe.Sizeof(Float64))
println(unsafe.Sizeof(Int64))

println(unsafe.Alignof(Float64))
println(unsafe.Alignof(Int64))
`
	testRun(t, src, false)
}
