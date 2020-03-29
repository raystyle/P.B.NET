package shellcode

import (
	"context"
	"testing"
	"time"

	"project/internal/patch/monkey"
	"project/internal/random"
	"project/internal/testsuite"
)

func TestDoUseless(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("panic", func(t *testing.T) {
		patchFunc := func() *random.Rand {
			panic(monkey.Panic)
		}
		pg := monkey.Patch(random.New, patchFunc)
		defer pg.Unpatch()
		ch := make(chan []byte, 5120)
		doUseless(context.Background(), ch)
	})

	t.Run("ctx.Done()", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(time.Second)
			cancel()
		}()
		ch := make(chan []byte, 16)
		doUseless(ctx, ch)
	})
}
