package anko

import (
	"testing"

	"project/internal/testsuite"

	_ "project/internal/anko/goroot"
	_ "project/internal/anko/thirdparty"
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
		const src = `

fmt = import("fmt")
 msgpack = import("github.com/vmihailenco/msgpack/v5")

 data,_ = msgpack.Marshal("acg")
fmt.Println(data)
println(data)

var asd = "asd"

 // msgpack = nil

 io = import("io")
 println(io.ErrUnexpectedEOF)

  acg = "acg"
  bb = func(){

   println(acg)

  acg = "acg2"
  }
  bb()

 println(acg)

  for i in []byte{1,2,3} {
     println(i)

  }





return true

`
		testRun(t, src, false, true)
	})

	t.Run("1p", func(t *testing.T) {
		const src = `range(3)`
		testRun(t, src, false, nil)
	})

	t.Run("2p", func(t *testing.T) {
		const src = `range(1, 3)`
		testRun(t, src, false, nil)
	})

	t.Run("3p", func(t *testing.T) {
		const src = `range(1, 10, 2)`
		testRun(t, src, false, nil)
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

func TestCoreArrayType(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
typ = arrayType(*new(int8), 4)
if typ.String() != "[4]int8" {
	return "invalid type"
}
return true
`
	testRun(t, src, false, true)
}

func TestCoreArray(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
a = array(*new(int8), 4)
// if typeOf(a) != "[4]int8" {
// 	panic("invalid type")
// }
// b = *a

b = a.Interface()

println(a[0])
a[0] = 123
println(a)
`
	testRun(t, src, false, nil)
}

func TestCoreSlice(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `




`
	testRun(t, src, false, nil)
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
