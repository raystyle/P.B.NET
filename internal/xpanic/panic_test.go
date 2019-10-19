package xpanic

import (
	"fmt"
	"testing"
)

func TestXpanic(t *testing.T) {
	defer func() {
		r := recover()
		fmt.Println("----start----")
		fmt.Println(Error(r, "TestXpanic"))
		fmt.Println("----end----")
	}()
	testPanic()
}

func testPanic() {
	var foo []int
	foo[0] = 0
}
