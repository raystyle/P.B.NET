package testsuite

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarkGoroutine(t *testing.T) {
	gm := MarkGoroutines(t)
	defer gm.Compare()

	c := make(chan struct{})
	go func() {
		c <- struct{}{}
	}()
	<-c
}

func TestMarkGoroutine_Leak(t *testing.T) {
	gm := MarkGoroutines(t)
	defer func() { require.Equal(t, 1, gm.calculate()) }()

	c := make(chan struct{})
	go func() {
		c <- struct{}{}
	}()
}

func TestMarkMemory(t *testing.T) {
	mm := MarkMemory(t)
	defer mm.Compare()
}

func TestMarkMemory_Leak(t *testing.T) {
	mm := MarkMemory(t)
	defer mm.Compare()
}
