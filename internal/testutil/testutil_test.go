package testutil

import (
	"fmt"
	"testing"
)

func TestIsDestroyed(t *testing.T) {
	a := 1
	fmt.Println(a)
	if !IsDestroyed(&a, 1) {
		t.Fatal("doesn't destroyed")
	}

	b := 2
	if IsDestroyed(&b, 1) {
		t.Fatal("destroyed")
	}
	fmt.Println(b)
}
