package api

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAPIError(t *testing.T) {
	t.Run("with error", func(t *testing.T) {
		err := newAPIError("test", "test reason", errors.New("test error"))
		require.EqualError(t, err, "test: test reason, because test error")
	})

	t.Run("without error", func(t *testing.T) {
		err := newAPIError("test", "test reason", nil)
		require.EqualError(t, err, "test: test reason")
	})
}

func TestNewAPIErrorf(t *testing.T) {
	t.Run("with error", func(t *testing.T) {
		err := newAPIErrorf("test", errors.New("test error"), "test reason format %s", "foo")
		require.EqualError(t, err, "test: test reason format foo, because test error")
	})

	t.Run("without error", func(t *testing.T) {
		err := newAPIErrorf("test", nil, "test reason format %s", "foo")
		require.EqualError(t, err, "test: test reason format foo")
	})
}
