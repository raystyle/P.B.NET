package system

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
)

func TestExecutableName(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		name, err := ExecutableName()
		require.NoError(t, err)
		t.Log(name)
	})

	t.Run("failed", func(t *testing.T) {
		patch := func() (string, error) {
			return "", monkey.Error
		}
		pg := monkey.Patch(os.Executable, patch)
		defer pg.Unpatch()

		name, err := ExecutableName()
		monkey.IsMonkeyError(t, err)
		require.Empty(t, name)
	})
}

func TestChangeCurrentDirectory(t *testing.T) {
	cd, err := os.Getwd()
	require.NoError(t, err)
	t.Log("current directory:", cd)
	defer func() {
		err = os.Chdir(cd)
		require.NoError(t, err)
	}()

	t.Run("ok", func(t *testing.T) {
		err = ChangeCurrentDirectory()
		require.NoError(t, err)

		dd, err := os.Getwd()
		require.NoError(t, err)
		t.Log("now directory:", dd)

		require.NotEqual(t, cd, dd)
	})

	t.Run("failed", func(t *testing.T) {
		patch := func() (string, error) {
			return "", monkey.Error
		}
		pg := monkey.Patch(os.Executable, patch)
		defer pg.Unpatch()

		err = ChangeCurrentDirectory()
		monkey.IsMonkeyError(t, err)
	})
}

func TestCheckError(t *testing.T) {
	t.Run("not nil", func(t *testing.T) {
		patch := func(int) {}
		pg := monkey.Patch(os.Exit, patch)
		defer pg.Unpatch()

		CheckError(errors.New("test error"))
	})

	t.Run("nil", func(t *testing.T) {
		CheckError(nil)
	})
}

func TestPrintError(t *testing.T) {
	patch := func(int) {}
	pg := monkey.Patch(os.Exit, patch)
	defer pg.Unpatch()

	PrintError("test error")
}
