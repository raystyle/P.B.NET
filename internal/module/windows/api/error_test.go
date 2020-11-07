// +build windows

package api

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAPIError(t *testing.T) {
	t.Run("with error", func(t *testing.T) {
		err := newError("test", errors.New("test error"), "test reason")
		require.EqualError(t, err, "test: test reason, because test error")
	})

	t.Run("without error", func(t *testing.T) {
		err := newError("test", nil, "test reason")
		require.EqualError(t, err, "test: test reason")
	})
}

func TestNewAPIErrorf(t *testing.T) {
	t.Run("with error", func(t *testing.T) {
		err := newErrorf("test", errors.New("test error"), "test reason format %s", "foo")
		require.EqualError(t, err, "test: test reason format foo, because test error")
	})

	t.Run("without error", func(t *testing.T) {
		err := newErrorf("test", nil, "test reason format %s", "foo")
		require.EqualError(t, err, "test: test reason format foo")
	})
}
