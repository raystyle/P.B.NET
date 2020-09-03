package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBogo(t *testing.T) {
	t.Run("common", func(t *testing.T) {
		bogo := NewBogo(4, time.Minute, nil)
		bogo.Wait()

		require.True(t, bogo.Compare())
	})

	t.Run("timeout", func(t *testing.T) {
		bogo := NewBogo(1024, time.Second, nil)
		bogo.Wait()

		require.False(t, bogo.Compare())
	})

	t.Run("invalid n or timeout", func(t *testing.T) {
		bogo := NewBogo(0, time.Hour, nil)
		bogo.Wait()

		require.True(t, bogo.Compare())
	})

	t.Run("cancel", func(t *testing.T) {
		bogo := NewBogo(1024, time.Second, nil)
		bogo.Stop()
		bogo.Wait()

		require.False(t, bogo.Compare())
	})
}
