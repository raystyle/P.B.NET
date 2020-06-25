package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateAndLoadSessionKey(t *testing.T) {
	keys, err := generateSessionKey()
	require.NoError(t, err)

	password := []byte("admin")
	file, err := encryptSessionKey(keys, password)
	require.NoError(t, err)

	decKeys, err := LoadSessionKey(file, password)
	require.NoError(t, err)
	require.Equal(t, keys, decKeys)

	const format = "\nprivate key: %X\nAES Key: %X\nAES IV: %X"
	t.Logf(format, keys[0], keys[1], keys[2])
}
