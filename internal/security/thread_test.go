package security

import (
	"context"
	"testing"
	"time"

	"project/internal/patch/monkey"
	"project/internal/random"
	"project/internal/testsuite"
)

func TestSwitchThread(t *testing.T) {
	SwitchThread()
}

func TestSchedule(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("panic", func(t *testing.T) {
		patch := func() *random.Rand {
			panic(monkey.Panic)
		}
		pg := monkey.Patch(random.NewRand, patch)
		defer pg.Unpatch()

		ch := make(chan []byte, 5120)
		schedule(context.Background(), ch)
	})

	t.Run("ctx.Done()", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(time.Second)
			cancel()
		}()

		ch := make(chan []byte, 16)
		schedule(ctx, ch)
	})
}

func BenchmarkSwitchThread(b *testing.B) {
	for i := 0; i < b.N; i++ {
		SwitchThread()
	}
}
