package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionKey(t *testing.T) {
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

func TestResetPassword(t *testing.T) {
	oldPwd := []byte("old")
	newPwd := []byte("new")

	keyFile, err := GenerateSessionKey(oldPwd)
	require.NoError(t, err)
	keys1, err := LoadSessionKey(keyFile, oldPwd)
	require.NoError(t, err)

	keyFile, err = ResetPassword(keyFile, oldPwd, newPwd)
	require.NoError(t, err)
	keys2, err := LoadSessionKey(keyFile, newPwd)
	require.NoError(t, err)

	require.Equal(t, keys1, keys2)
}
