package rand

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReader_Read(t *testing.T) {
	b1 := make([]byte, 32)
	_, err := io.ReadFull(Reader, b1)
	require.NoError(t, err)
	b2 := make([]byte, 32)
	_, err = io.ReadFull(Reader, b2)
	require.NoError(t, err)
	require.NotEqual(t, b1, b2)
}
