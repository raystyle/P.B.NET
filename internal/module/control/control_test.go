package control

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestController(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	ctrl := NewController(context.Background())
	require.Equal(t, StateRunning, ctrl.State())

	ctrl.Pause()
	require.Equal(t, StatePaused, ctrl.State())

	go func() {
		time.Sleep(2 * time.Second)
		ctrl.Continue()
	}()

	now := time.Now()
	ctrl.Paused()
	require.True(t, time.Since(now) > time.Second)
	require.Equal(t, StateRunning, ctrl.State())
}

func TestController_Continue(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	ctx := context.Background()

	t.Run("continue but not paused", func(t *testing.T) {
		ctrl := NewController(ctx)
		ctrl.Continue()
	})

	t.Run(" simulate continue too fast", func(t *testing.T) {
		ctrl := NewController(ctx)
		fakeState := StatePaused
		ctrl.state = &fakeState

		ctrl.Continue()
		ctrl.Continue()
	})
}

func TestController_Pause(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	ctx := context.Background()

	t.Run("not paused", func(t *testing.T) {
		ctrl := NewController(ctx)

		now := time.Now()
		ctrl.Paused()
		require.True(t, time.Since(now) < time.Second)
		require.Equal(t, StateRunning, ctrl.State())
	})

	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		ctrl := NewController(ctx)
		ctrl.Pause()
		require.Equal(t, StatePaused, ctrl.State())

		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()

		now := time.Now()
		ctrl.Paused()
		require.True(t, time.Since(now) > time.Second)
		require.Equal(t, StateCancel, ctrl.State())

		ctrl.Paused()
	})
}
