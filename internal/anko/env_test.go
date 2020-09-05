package anko

import (
	"testing"

	"project/internal/testsuite"
)

func TestEnv_Global(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const src = `
val = make(struct{
A string,
B string
})



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
		const src = `eval("a -- a")`
		testRun(t, src, true, nil)
	})

	t.Run("eval with error", func(t *testing.T) {
		const src = "eval(`" + `
a = 10
println(a)

println(b)
` + "`)"
		testRun(t, src, true, nil)
	})
}
