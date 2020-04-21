package rand

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
)

func TestReader_Read(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		b1 := make([]byte, 32)
		_, err := io.ReadFull(Reader, b1)
		require.NoError(t, err)
		b2 := make([]byte, 32)
		_, err = io.ReadFull(Reader, b2)
		require.NoError(t, err)
		require.NotEqual(t, b1, b2)
	})

	t.Run("failed", func(t *testing.T) {
		patchFunc := func(_ io.Reader, _ []byte) (int, error) {
			return 0, monkey.Error
		}
		pg := monkey.Patch(io.ReadFull, patchFunc)
		defer pg.Unpatch()
		_, err := Reader.Read(make([]byte, 1024))
		monkey.IsMonkeyError(t, err)
	})
}
