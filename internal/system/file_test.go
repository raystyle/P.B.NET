package system

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestOpenFile(t *testing.T) {
	const (
		flag = os.O_WRONLY | os.O_CREATE
		perm = 0600
	)

	t.Run("ok", func(t *testing.T) {
		const name = "testdata/of.dat"

		file, err := OpenFile(name, flag, perm)
		require.NoError(t, err)

		err = file.Close()
		require.NoError(t, err)

		err = os.Remove(name)
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		file, err := OpenFile("testdata/<</file", flag, perm)
		require.Error(t, err)
		require.Nil(t, file)
	})
}

func TestWriteFile(t *testing.T) {
	testdata := testsuite.Bytes()

	t.Run("ok", func(t *testing.T) {
		const name = "testdata/wf.dat"

		err := WriteFile(name, testdata)
		require.NoError(t, err)

		err = os.Remove(name)
		require.NoError(t, err)
	})

	t.Run("invalid path", func(t *testing.T) {
		err := WriteFile("testdata/<</file", testdata)
		require.Error(t, err)
	})
}

func TestIsExist(t *testing.T) {
	t.Run("exist", func(t *testing.T) {
		exist, err := IsExist("testdata")
		require.NoError(t, err)
		require.True(t, exist)
	})

	t.Run("is not exist", func(t *testing.T) {
		exist, err := IsExist("not")
		require.NoError(t, err)
		require.False(t, exist)
	})

	t.Run("invalid path", func(t *testing.T) {
		exist, err := IsExist("testdata/<</file")
		require.Error(t, err)
		require.False(t, exist)
	})
}

func TestIsNotExist(t *testing.T) {
	t.Run("is not exist", func(t *testing.T) {
		notExist, err := IsNotExist("not")
		require.NoError(t, err)
		require.True(t, notExist)
	})

	t.Run("exist", func(t *testing.T) {
		notExist, err := IsNotExist("testdata")
		require.NoError(t, err)
		require.False(t, notExist)
	})

	t.Run("invalid path", func(t *testing.T) {
		notExist, err := IsNotExist("testdata/<</file")
		require.Error(t, err)
		require.False(t, notExist)
	})
}
