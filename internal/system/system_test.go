package system

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
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
	err := os.MkdirAll("testdata", 0750)
	require.NoError(t, err)

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

func TestGetConnHandle(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		handle, err := GetConnHandle(os.Stdout)
		require.NoError(t, err)
		require.NotZero(t, handle)
		t.Log("handle:", handle)
	})

	t.Run("failed to call SyscallConn", func(t *testing.T) {
		patch := func(interface{}) (syscall.RawConn, error) {
			return nil, monkey.Error
		}
		pg := monkey.PatchInstanceMethod(os.Stdout, "SyscallConn", patch)
		defer pg.Unpatch()

		handle, err := GetConnHandle(os.Stdout)
		monkey.IsMonkeyError(t, err)
		require.Zero(t, handle)
	})

	t.Run("failed to call Control", func(t *testing.T) {
		rawConn, err := os.Stdout.SyscallConn()
		require.NoError(t, err)

		patch := func(interface{}, func(fd uintptr)) error {
			return monkey.Error
		}
		pg := monkey.PatchInstanceMethod(rawConn, "Control", patch)
		defer pg.Unpatch()

		handle, err := GetConnHandle(os.Stdout)
		monkey.IsMonkeyError(t, err)
		require.Zero(t, handle)
	})
}

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
