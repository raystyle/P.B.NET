package anko

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/mattn/anko/env"
	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestCoreType(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
// --------uint--------

v = new(uint)
if typeOf(v) != "*uint" {
	return "not *uint type"
}

v = new(uint8)
if typeOf(v) != "*uint8" {
	return "not *uint8 type"
}

v = new(uint16)
if typeOf(v) != "*uint16" {
	return "not *uint16 type"
}

v = new(uint32)
if typeOf(v) != "*uint32" {
	return "not *uint32 type"
}

v = new(uint64)
if typeOf(v) != "*uint64" {
	return "not *uint64 type"
}

// --------int--------

v = new(int)
if typeOf(v) != "*int" {
	return "not *int type"
}

v = new(int8)
if typeOf(v) != "*int8" {
	return "not *int8 type"
}

v = new(int16)
if typeOf(v) != "*int16" {
	return "not *int16 type"
}

v = new(int32)
if typeOf(v) != "*int32" {
	return "not *int32 type"
}

v = new(int64)
if typeOf(v) != "*int64" {
	return "not *int64 type"
}

// --------other--------

v = new(byte)
if typeOf(v) != "*uint8" {
	return "not *uint8 type"
}

v = new(rune)
if typeOf(v) != "*int32" {
	return "not *int32 type"
}

v = new(uintptr)
if typeOf(v) != "*uintptr" {
	return "not *uintptr type"
}

v = new(float32)
if typeOf(v) != "*float32" {
	return "not *float32 type"
}

v = new(float64)
if typeOf(v) != "*float64" {
	return "not *float64 type"
}

return true
`
	testRun(t, src, false, true)
}

func TestCoreKeys(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
m = {"foo": "bar", "bar": "baz"}
println(keys(m))
return true
`
	testRun(t, src, false, true)
}

func TestCoreRange(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("no parameter", func(t *testing.T) {
		const src = `range()`
		testRun(t, src, true, nil)
	})

	t.Run("1p", func(t *testing.T) {
		const src = `range(3)`
		testRun(t, src, false, []int64{0, 1, 2})
	})

	t.Run("2p", func(t *testing.T) {
		const src = `range(1, 3)`
		testRun(t, src, false, []int64{1, 2})
	})

	t.Run("3p", func(t *testing.T) {
		const src = `range(1, 10, 2)`
		testRun(t, src, false, []int64{1, 3, 5, 7, 9})
	})

	t.Run("3p-zero step", func(t *testing.T) {
		const src = `range(1, 10, 0)`
		testRun(t, src, true, nil)
	})

	t.Run("4p", func(t *testing.T) {
		const src = `range(1, 2, 3, 4)`
		testRun(t, src, true, nil)
	})
}

func TestCoreInstance(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
sa = make(type sa, make(struct{
 A string,
 B string
}))

i1 = instance(sa)
i1.A = "acg"

i2 = instance(make(sa))
i2.A = "abc"
i2.B = "bbb"

if i1.A != "acg" {
	return "invalid i1.A"
}
if i2.A != "abc" {
	return "invalid i2.A"
}
if i2.B != "bbb" {
	return "invalid i2.B"
}
return true
`
	testRun(t, src, false, true)
}

func TestCoreArrayType(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
typ = arrayType(make(int8), 4)
if typ.String() != "[4]int8" {
	return "invalid type"
}

// i1 = make(typ) undefined
// i1 = new(typ) undefined
i1 = instance(typ)
println(i1)

return true
`
	testRun(t, src, false, true)
}

func TestCoreArray(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
a = array(make(int8), 4)
if typeOf(a) != "*[4]int8" {
	return "not *[4]int8 type"
}
a = *a

p1 = &a
printf("%p", p1)

a[1] = 123
if a[1] != 123 {
	return "invalid array value"
}

p2 = &a
printf("%p", p2)

if p1 != p2 {
	return "address changed"
}
return true
`
	testRun(t, src, false, true)
}

func TestCoreSlice(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
s = slice(array(make(int8), 4))
if typeOf(s) != "[]int8" {
	return "not []int8 type"
}
println(s)
return true
`
	testRun(t, src, false, true)
}

func TestCoreTypeOf(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
v = 10
if typeOf(v) != "int64"{
	return "not int64 type"
}
return true
`
	testRun(t, src, false, true)
}

func TestCoreKindOf(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("int64", func(t *testing.T) {
		const src = `
v = 10
if kindOf(v) != "int64" {
	return "not int64 kind"
}
return true
`
		testRun(t, src, false, true)
	})

	t.Run("nil", func(t *testing.T) {
		const src = `
v = nil
if kindOf(v) != "nil kind" {
	return "not nil kind"
}
return true
`
		testRun(t, src, false, true)
	})
}

func TestDefineCoreType(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	e := env.NewEnv()
	patch := func(interface{}, string, interface{}) error {
		return monkey.Error
	}
	pg := monkey.PatchInstanceMethod(e, "DefineType", patch)
	defer pg.Unpatch()

	defer testsuite.DeferForPanic(t)
	defineCoreType(e)
}

func TestDefineCoreFunc(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	e := env.NewEnv()
	patch := func(interface{}, string, interface{}) error {
		return monkey.Error
	}
	pg := monkey.PatchInstanceMethod(e, "Define", patch)
	defer pg.Unpatch()

	defer testsuite.DeferForPanic(t)
	defineCoreFunc(e)
}

func TestModule(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	e := NewEnv()

	const src = `
module a { b = 1 }
c = a
d = a
d.b = 2

println(c)
println(d)


return true
`
	stmt := testParseSrc(t, src)

	val, err := Run(e, stmt)
	require.NoError(t, err)
	require.Equal(t, true, val)

	e.Close()

	e.env.Copy()

	ne := e.env
	testsuite.IsDestroyed(t, e)
	testsuite.IsDestroyed(t, ne)
	testsuite.IsDestroyed(t, stmt)
}

func TestAnkoModule(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	e := NewEnv()

	const src = `
module Test {
	_inner1 = "acg"
	_inner2 = 1

	func Input(v1, v2) {
		_inner1 = v1
		_inner2 = v2
		return
	}

	func Output() {
		return _inner1, _inner2
	}
}
func New() {
	var mod = Test
	return mod
}
return true
`
	stmt := testParseSrc(t, src)

	val, err := Run(e, stmt)
	require.NoError(t, err)
	require.Equal(t, true, val)

	// get New function
	newFn, err := e.Get("New")
	require.NoError(t, err)
	create := newFn.(func(context.Context) (reflect.Value, reflect.Value))

	ctx := context.Background()

	f := func() {
		// create instance
		mod, re := create(ctx)
		require.True(t, re.IsNil())
		module := mod.Interface().(*env.Env)

		// get output function
		outputFn, err := module.Get("Output")
		require.NoError(t, err)
		output := outputFn.(func(context.Context) (reflect.Value, reflect.Value))

		// call output
		ret, re := output(ctx)
		require.True(t, re.IsNil())
		i1 := ret.Index(0).Interface()
		i2 := ret.Index(1).Interface()

		require.Equal(t, "acg", i1)
		require.Equal(t, int64(1), i2)

		fmt.Println(i1)
		fmt.Println(i2)

		// get input function
		inputFn, err := module.Get("Input")
		require.NoError(t, err)
		input := inputFn.(func(context.Context, reflect.Value, reflect.Value) (reflect.Value, reflect.Value))

		// call input
		ret, re = input(ctx, reflect.ValueOf("aaa"), reflect.ValueOf(int64(2)))
		require.True(t, re.IsNil())
		require.True(t, ret.IsNil())

		ret, re = output(ctx)
		require.True(t, re.IsNil())
		i1 = ret.Index(0).Interface()
		i2 = ret.Index(1).Interface()

		require.Equal(t, "aaa", i1)
		require.Equal(t, int64(2), i2)
	}
	f()
	f()

	e.Close()

	ne := e.env
	testsuite.IsDestroyed(t, e)
	testsuite.IsDestroyed(t, ne)
	testsuite.IsDestroyed(t, stmt)
}
