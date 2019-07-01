package controller

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_KeyGen(t *testing.T) {
	path := os.TempDir() + "/ctrl.key"
	const password = "0123456789012"
	err := Gen_CTRL_Keys(path, password)
	require.Nil(t, err, err)
	_, err = Load_CTRL_Keys(path, password)
	require.Nil(t, err, err)
	err = os.Remove(path)
	require.Nil(t, err, err)
}
