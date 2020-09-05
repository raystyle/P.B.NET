package anko

import (
	"testing"

	"github.com/mattn/anko/env"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestConvert(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
v = 123

// --------uint--------

cv = uint(v)
if typeOf(cv) != "uint" {
	return "uint"
}

cv = uint8(v)
if typeOf(cv) != "uint8" {
	return "uint8"
}

cv = uint16(v)
if typeOf(cv) != "uint16" {
	return "uint16"
}

cv = uint32(v)
if typeOf(cv) != "uint32" {
	return "uint32"
}

cv = uint64(v)
if typeOf(cv) != "uint64" {
	return "uint64"
}

// --------int--------

cv = int(v)
if typeOf(cv) != "int" {
	return "int"
}

cv = int8(v)
if typeOf(cv) != "int8" {
	return "int8"
}

cv = int16(v)
if typeOf(cv) != "int16" {
	return "int16"
}

cv = int32(v)
if typeOf(cv) != "int32" {
	return "int32"
}

cv = int64(v)
if typeOf(cv) != "int64" {
	return "int64"
}

// --------other--------

cv = byte(v)
if typeOf(cv) != "uint8" {
	return "byte"
}

cv = rune(v)
if typeOf(cv) != "int32" {
	return "rune"
}

cv = uintptr(v)
if typeOf(cv) != "uintptr" {
	return "uintptr"
}

cv = float32(v)
if typeOf(cv) != "float32" {
	return "float32"
}

cv = float64(v)
if typeOf(cv) != "float64" {
	return "float64"
}

// --------string--------

cv = string([]byte{97, 98, 99})
if typeOf(cv) != "string" {
	return "byte slice"
}
if cv != "abc" {
	return "not abc"
}

cv = string([]rune{97, 98, 99})
if typeOf(cv) != "string" {
	return "byte slice"
}
if cv != "abc" {
	return "not abc"
}

// --------byte slice--------

cv = byteSlice("abc")
if typeOf(cv) != "[]uint8" {
	return "[]byte"
}
if cv != []byte{97, 98, 99} {
	return "not []byte abc"
}

// --------rune slice--------

cv = runeSlice("abc")
if typeOf(cv) != "[]int32" {
	return "[]rune"
}
if cv != []rune{97, 98, 99} {
	return "not []byte abc"
}

return true
`
	testRun(t, src, false, true)
}

func TestDefineConvert(t *testing.T) {
	e := env.NewEnv()
	patch := func(interface{}, string, interface{}) error {
		return monkey.Error
	}
	pg := monkey.PatchInstanceMethod(e, "Define", patch)
	defer pg.Unpatch()

	defer testsuite.DeferForPanic(t)
	defineConvert(e)
}
