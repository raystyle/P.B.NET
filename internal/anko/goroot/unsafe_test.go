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

	const src = `
unsafe = import("unsafe")
reflect = import("reflect")

val = 256

// sizeOf
println(unsafe.Sizeof(val))

// AlignOf
println(unsafe.Alignof(val))

// convert to struct
// p := (*testStruct)(unsafe.Pointer(&Int64))

ss = make(struct {
	A int32,
	B int32
})

p = unsafe.Convert(&val, ss)

pv = p.Interface()
println(pv)

// byte order
if !(pv.A == 256 || pv.B == 256) {
	println(val)
	return false
}

// cover
p.Set(reflect.ValueOf(ss))
if val != 0 {
	println(val)
	return false
}


// make [8]byte and test ConvertWithType
//
// p := (*[8]byte)(unsafe.Pointer(&Int64))

val = 256
typ = unsafe.ArrayOf(*new(byte), 8)
p = unsafe.ConvertWithType(&val, typ)

aaa = p.Interface()
println(aaa[1])
// can't call aaa[1] = 1

slice = unsafe.ByteArrayToSlice(p)

println(slice[:4], slice[4:])

// cover
for i = 0; i < 8; i++ {
	slice[i] = 0
}
if val != 0 {
	println(val)
	return false
}

return true
`
	testRun(t, src, false, true)
}
