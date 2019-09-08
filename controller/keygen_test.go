package controller

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyGen(t *testing.T) {
	path := os.TempDir() + "/ctrl.key"
	const password = "0123456789012"
	err := GenCtrlKeys(path, password)
	require.NoError(t, err)
	_, err = loadCtrlKeys(path, password)
	require.NoError(t, err)
	err = os.Remove(path)
	require.NoError(t, err)
}
