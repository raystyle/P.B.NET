package xpanic

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
)

func TestPrintStack(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		testFuncA()
	})

	t.Run("skip > max depth", func(t *testing.T) {
		b := new(bytes.Buffer)
		PrintStack(b, maxDepth+1)

		fmt.Println("-----begin-----")
		fmt.Print(b)
		fmt.Println("-----end-----")
	})

	t.Run("panic", func(t *testing.T) {
		patch := func(uintptr) *runtime.Func {
			panic(monkey.Panic)
		}
		pg := monkey.Patch(runtime.FuncForPC, patch)
		defer pg.Unpatch()

		testLogPanic()
	})
}

func testFuncA() {
	testFuncB()
}

func testFuncB() {
	testFuncC()
}

func testFuncC() {
	// appear some error
	testLogPanic()
}

func testLogPanic() {
	b := new(bytes.Buffer)
	PrintStack(b, 0)

	fmt.Println("-----begin-----")
	fmt.Print(b)
	fmt.Println("-----end-----")
}

func TestError(t *testing.T) {
	defer func() {
		r := recover()
		fmt.Println("-----begin-----")
		fmt.Print(Error(r, "TestError"))
		fmt.Println("-----end-----")
	}()

	testPanic()
}

func TestLog(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				b := Log(r, "testLog")
				require.NotNil(t, b)
			}
		}()

		testPanic()
	}()
	wg.Wait()
}

func TestUnknown(t *testing.T) {
	patch := func(uintptr) *runtime.Func {
		return nil
	}
	pg := monkey.Patch(runtime.FuncForPC, patch)
	defer pg.Unpatch()

	defer func() {
		r := recover()
		fmt.Println("-----begin-----")
		fmt.Print(Error(r, "TestUnknown"))
		fmt.Println("-----end-----")
	}()

	testPanic()
}

func testPanic() {
	var foo []int
	foo[0] = 0
}
