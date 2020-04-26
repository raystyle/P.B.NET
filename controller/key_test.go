package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateAndLoadSessionKey(t *testing.T) {
	password := []byte("pbnet")
	data, err := GenerateSessionKey(password)
	require.NoError(t, err)
	keys, err := loadSessionKey(data, password)
	require.NoError(t, err)
	const format = "\nprivate key: %X\nAES Key: %X\nAES IV: %X"
	t.Logf(format, keys[0], keys[1], keys[2])
}
