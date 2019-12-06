package controller

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateSessionKey(t *testing.T) {
	path := os.TempDir() + "/session.key"
	password := []byte("pbnet")
	err := GenerateSessionKey(path, password)
	require.NoError(t, err)
	defer func() {
		err = os.Remove(path)
		require.NoError(t, err)
	}()
	keys, err := loadSessionKey(path, password)
	require.NoError(t, err)
	t.Logf("private key: %X\nAES Key: %X\nAES IV: %X",
		keys[0], keys[1], keys[2])
}
