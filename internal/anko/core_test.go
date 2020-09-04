package anko

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

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

func TestBasicType(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
a = []int8{1, 2}
a[0] += 1
a += 3
println(a, typeOf(a))
a = a[:2]

a = make([]int8, 0, 3)
a += 4
println(a, typeOf(a))

a = []int16{1, 2}
a[0] += 1
println(a, typeOf(a))

a = []int32{1, 2}
a[0] += 1
println(a, typeOf(a))

a = []int64{1, 2}
a[0] += 1
println(a, typeOf(a))

a = [1, 2]
a[0] += 1
println(a, typeOf(a))

a = []uint8{1, 2}
a[0] += 1
println(a, typeOf(a))

a = []uint16{1, 2}
a[0] += 1
println(a, typeOf(a))

a = []uint32{1, 2}
a[0] += 1
println(a, typeOf(a))

a = []uint64{1, 2}
a[0] += 1
println(a, typeOf(a))

a = []uintptr{1, 2}
a[0] += 1
println(a, typeOf(a))

a = 1<<64 - 1
println(a, typeOf(a))

println(1<<64 - 1)

b = 1<<64 - 1
`
	testRun(t, src, false)
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

	t.Run("no parameter", func(t *testing.T) {
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

func TestCoreArrayType(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
typ = arrayType(*new(int8), 4)
if typ.String() != "[4]int8" {
	panic("invalid type")
}
`
	testRun(t, src, false)
}

func TestCoreArray(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
a = array(*new(asd), 4)
// if typeOf(a) != "[4]int8" {
// 	panic("invalid type")
// }
// b = *a

a[0] = 123
println(a)
`
	testRun(t, src, false)
}

func TestCoreSlice(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
a = 10
println(typeOf(a))
`
	testRun(t, src, false)
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
