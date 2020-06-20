package anko

import (
	"testing"

	"project/internal/testsuite"

	_ "project/internal/anko/goroot"
	_ "project/internal/anko/project"
	_ "project/internal/anko/thirdparty"
)

func TestGoRootUnsafe(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
unsafe = import("unsafe")
reflect = import("reflect")

Float64 = 0.614
Int64 = 123

println(Int64)

cc = unsafe.Convert(reflect.TypeOf(Float64), reflect.TypeOf(Int64))

cc(reflect.ValueOf(Float64), reflect.ValueOf(Int64))

println(Int64)

`
	testRun(t, src, false)
}
