package goroot

import (
	"fmt"
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"

	"project/internal/anko"
	"project/internal/testsuite"
)

func testRun(t *testing.T, s string, fail bool, expected interface{}) {
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
	require.Equal(t, expected, val)

	testsuite.IsDestroyed(t, env)
	testsuite.IsDestroyed(t, stmt)
}

func TestUnsafeAboutStruct(t *testing.T) {
	type s struct {
		A int32
		B int32
	}
	val := int64(256)

	aa := (*s)(unsafe.Pointer(&val))
	fmt.Println(aa.A)
	fmt.Println(aa.B)

	n := *(*[8]byte)(unsafe.Pointer(&val))
	fmt.Println(n)
}

func TestUnsafe(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("sizeOf and alignOf", func(t *testing.T) {
		const src = `
unsafe = import("unsafe")

val = 256

size = unsafe.Sizeof(val)
if size != 8 {
	return size
}

align = unsafe.Alignof(val)
if align != 8 {
	return align
}

return true
`
		testRun(t, src, false, true)
	})

	t.Run("convert to struct", func(t *testing.T) {
		// convert to struct
		// like these golang code
		// p := (*testStruct)(unsafe.Pointer(&Int64))
		const src = `
unsafe = import("unsafe")
reflect = import("reflect")

val = 256
ss = make(struct {
	A int32,
	B int32
})
p = unsafe.Convert(&val, ss)

pv = p.Interface()
println(pv.A, pv.B)

// byte order
if !(pv.A == 256 || pv.B == 256) {
	return val
}

// cover memory
p.Set(reflect.ValueOf(ss))
if val != 0 {
	return val
}

return true
`
		testRun(t, src, false, true)
	})

	t.Run("convert to byte slice", func(t *testing.T) {
		// make [8]byte and test ConvertWithType
		// like these golang code
		// p := (*[8]byte)(unsafe.Pointer(&Int64))
		const src = `
unsafe = import("unsafe")
reflect = import("reflect")

val = 256
typ = unsafe.ArrayOf(*new(byte), 8)
p = unsafe.ConvertWithType(&val, typ)

pv = p.Interface()
println(pv[1])
// can't call pv[1] = 1

bs = unsafe.ByteArrayToSlice(p)
println(bs[:4], bs[4:])

// cover memory
for i = 0; i < 8; i++ {
	bs[i] = 0
}
if val != 0 {
	return val
}

return true
`
		testRun(t, src, false, true)
	})
}
