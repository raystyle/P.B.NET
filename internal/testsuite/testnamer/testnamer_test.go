package testnamer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestNewNamer(t *testing.T) {
	namer := NewNamer()

	err := namer.Load(nil)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		word, err := namer.Generate(nil)
		require.NoError(t, err)
		t.Log(word)
	}

	testsuite.IsDestroyed(t, namer)
}

func TestNewNamerWithLoadFailed(t *testing.T) {
	namer := NewNamerWithLoadFailed()

	err := namer.Load(nil)
	require.Error(t, err)

	testsuite.IsDestroyed(t, namer)
}

func TestNewNamerWithGenerateFailed(t *testing.T) {
	namer := NewNamerWithGenerateFailed()

	err := namer.Load(nil)
	require.NoError(t, err)

	word, err := namer.Generate(nil)
	require.Error(t, err)
	require.Zero(t, word)

	testsuite.IsDestroyed(t, namer)
}
