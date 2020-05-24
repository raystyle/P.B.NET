package testsuite

import (
	"fmt"
	"io/ioutil"
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
	defer func() {
		delta := gm.calculate()
		require.Equal(t, 1, delta)
	}()

	c := make(chan struct{})
	go func() {
		c <- struct{}{}
	}()
}

func TestIsDestroyed(t *testing.T) {
	t.Run("destroyed", func(t *testing.T) {
		obj := 1

		n, err := fmt.Fprintln(ioutil.Discard, obj)
		require.Equal(t, n, 2)
		require.NoError(t, err)

		if !Destroyed(&obj) {
			t.Fatal("doesn't destroyed")
		}
	})

	t.Run("doesn't destroyed", func(t *testing.T) {
		obj := 2

		if Destroyed(&obj) {
			t.Fatal("destroyed")
		}

		n, err := fmt.Fprintln(ioutil.Discard, obj)
		require.Equal(t, n, 2)
		require.NoError(t, err)
	})

	t.Run("is destroyed", func(t *testing.T) {
		obj := 3

		n, err := fmt.Fprintln(ioutil.Discard, obj)
		require.Equal(t, n, 2)
		require.NoError(t, err)

		IsDestroyed(t, &obj)
	})
}

func TestMarkMemory(t *testing.T) {
	mm := MarkMemory(t)
	defer mm.Compare()
}

func TestMarkMemory_Leak(t *testing.T) {
	mm := MarkMemory(t)
	defer mm.Compare()
}
