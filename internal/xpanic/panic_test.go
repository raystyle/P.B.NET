package xpanic

import (
	"fmt"
	"testing"
)

func Test_Print(t *testing.T) {
	defer func() {
		r := recover()
		fmt.Println(Print(r))
		fmt.Println(Error("test panic:", r))
	}()
	test_panic()
}

func test_panic() {
	var foo []int
	foo[0] = 0
}
