package security

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLimitReadAll(t *testing.T) {
	t.Run("read = size", func(t *testing.T) {
		buf := bytes.NewBuffer(make([]byte, 1024))
		b, err := LimitReadAll(buf, 1024)
		require.NoError(t, err)
		require.Equal(t, make([]byte, 1024), b)
	})

	t.Run("read < size", func(t *testing.T) {
		buf := bytes.NewBuffer(make([]byte, 1024))
		b, err := LimitReadAll(buf, 512)
		require.NoError(t, err)
		require.Equal(t, make([]byte, 512), b)
	})

	t.Run("read > size", func(t *testing.T) {
		buf := bytes.NewBuffer(make([]byte, 1024))
		b, err := LimitReadAll(buf, 2048)
		require.NoError(t, err)
		require.Equal(t, make([]byte, 1024), b)
	})
}
