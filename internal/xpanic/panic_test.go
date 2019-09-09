package xpanic

import (
	"fmt"
	"testing"
)

func TestXpanic(t *testing.T) {
	defer func() {
		r := recover()
		fmt.Println(Sprint(r))
		fmt.Println(Error("test panic:", r))
	}()
	testPanic()
}

func testPanic() {
	var foo []int
	foo[0] = 0
}
