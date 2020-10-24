package security

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLimitReadAll(t *testing.T) {
	t.Run("limitation = size", func(t *testing.T) {
		buf := bytes.NewBuffer(make([]byte, 1024))
		b, err := LimitReadAll(buf, 1024)
		require.NoError(t, err)
		require.Equal(t, make([]byte, 1024), b)
	})

	t.Run("limitation < size", func(t *testing.T) {
		buf := bytes.NewBuffer(make([]byte, 1024))
		b, err := LimitReadAll(buf, 512)
		require.NoError(t, err)
		require.Equal(t, make([]byte, 512), b)
	})

	t.Run("limitation > size", func(t *testing.T) {
		buf := bytes.NewBuffer(make([]byte, 1024))
		b, err := LimitReadAll(buf, 2048)
		require.NoError(t, err)
		require.Equal(t, make([]byte, 1024), b)
	})
}

func TestLimitReadAllWithError(t *testing.T) {
	t.Run("limitation = size", func(t *testing.T) {
		buf := bytes.NewBuffer(make([]byte, 123))
		b, err := LimitReadAllWithError(buf, 123)
		require.NoError(t, err)
		require.Equal(t, make([]byte, 123), b)
	})

	t.Run("limitation < size", func(t *testing.T) {
		buf := bytes.NewBuffer(make([]byte, 456))
		b, err := LimitReadAllWithError(buf, 123)
		require.Error(t, err)
		require.Equal(t, make([]byte, 123), b)
	})

	t.Run("limitation > size", func(t *testing.T) {
		buf := bytes.NewBuffer(make([]byte, 123))
		b, err := LimitReadAllWithError(buf, 456)
		require.NoError(t, err)
		require.Equal(t, make([]byte, 123), b)
	})
}
