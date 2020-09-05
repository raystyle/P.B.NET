package anko

import (
	"testing"

	"project/internal/testsuite"
)

func TestConvert(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `

 bbb = uint8(16)

println(&bbb)

bbb +=1

println(&bbb)

a = uint8(256)
println(a)
println(typeOf(a))
a = int(a)
println(typeOf(a))

b = new(uint8)

*b = uint8(8)

println(*b)

asd = "abc"
println(byteSlice(asd))

`
	testRun(t, src, false)
}
