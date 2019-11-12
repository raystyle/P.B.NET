package xpanic

import (
	"fmt"
	"testing"
)

func TestXPanic(t *testing.T) {
	defer func() {
		r := recover()
		fmt.Println("-----begin-----")
		fmt.Println(Error(r, "TestXPanic"))
		fmt.Println("-----end-----")
	}()
	testPanic()
}

func testPanic() {
	var foo []int
	foo[0] = 0
}
