package xpanic

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/bouk/monkey"
)

func testPanic() {
	var foo []int
	foo[0] = 0
}

func TestError(t *testing.T) {
	defer func() {
		r := recover()
		fmt.Println("-----begin-----")
		fmt.Println(Error(r, "TestError"))
		fmt.Println("-----end-----")
	}()
	testPanic()
}

func TestUnknown(t *testing.T) {
	patchFunc := func(uintptr) *runtime.Func {
		return nil
	}
	pg := monkey.Patch(runtime.FuncForPC, patchFunc)
	defer pg.Unpatch()

	defer func() {
		r := recover()
		fmt.Println(Error(r, "TestUnknown"))
	}()
	testPanic()
}
