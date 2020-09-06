package anko

import (
	"testing"

	"project/internal/testsuite"

	_ "project/internal/anko/goroot"
	_ "project/internal/anko/thirdparty"
)

func TestEnv(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

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
}

func TestEnv_eval(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("ok", func(t *testing.T) {
		const src = `
a = "out"
eval('a = "in eval"')
println(a)
return true
`
		testRun(t, src, false, true)
	})

	t.Run("invalid source", func(t *testing.T) {
		const src = `
val, err = eval("a -- a")
if err == nil {
	return "no error"
}
println("val:", val)
println("error:", err)
return true
`
		testRun(t, src, false, true)
	})

	t.Run("eval with error", func(t *testing.T) {
		const src = `
src = "a = 10\n"
src += "println(a)\n"
src += "println(b)\n"
val, err = eval(src)
if err == nil {
	return "no error"
}
println("val:", val)
println("error:", err)
return true
`
		testRun(t, src, false, true)
	})

	t.Run("nest", func(t *testing.T) {
		const src = `
a = "out"
eval("eval('a = \"nest eval\"')")
println(a)
return true
`
		testRun(t, src, false, true)
	})
}
