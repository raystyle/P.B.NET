package xpanic

import (
	"fmt"
	"testing"
)

func Test_Print(t *testing.T) {
	defer func() {
		fmt.Println(Print(recover()))
	}()
	test_panic()
}

func test_panic() {
	var foo []int
	foo[0] = 0
}
