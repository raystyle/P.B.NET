package system

import (
	"log"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
)

func TestGetConnHandle(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		handle, err := GetConnHandle(os.Stdout)
		require.NoError(t, err)
		require.NotZero(t, handle)
		t.Log("handle:", handle)
	})

	t.Run("failed to call SyscallConn", func(t *testing.T) {
		patchFunc := func(interface{}) (syscall.RawConn, error) {
			return nil, monkey.Error
		}
		pg := monkey.PatchInstanceMethod(os.Stdout, "SyscallConn", patchFunc)
		defer pg.Unpatch()

		handle, err := GetConnHandle(os.Stdout)
		monkey.IsMonkeyError(t, err)
		require.Zero(t, handle)
	})

	t.Run("failed to call Control", func(t *testing.T) {
		rawConn, err := os.Stdout.SyscallConn()
		require.NoError(t, err)

		patchFunc := func(interface{}, func(fd uintptr)) error {
			return monkey.Error
		}
		pg := monkey.PatchInstanceMethod(rawConn, "Control", patchFunc)
		defer pg.Unpatch()

		handle, err := GetConnHandle(os.Stdout)
		monkey.IsMonkeyError(t, err)
		require.Zero(t, handle)
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

	err = ChangeCurrentDirectory()
	require.NoError(t, err)

	dd, err := os.Getwd()
	require.NoError(t, err)
	t.Log("now directory:", dd)

	require.NotEqual(t, cd, dd)

	// failed
	patchFunc := func() (string, error) {
		return "", monkey.Error
	}
	pg := monkey.Patch(os.Executable, patchFunc)
	defer pg.Unpatch()

	err = ChangeCurrentDirectory()
	monkey.IsMonkeyError(t, err)
}

func TestSetErrorLogger(t *testing.T) {
	_ = os.Mkdir("testdata", 0750)

	const name = "testdata/test.err"

	file, err := SetErrorLogger(name)
	require.NoError(t, err)
	defer func() {
		err = file.Close()
		require.NoError(t, err)
		err = os.Remove(name)
		require.NoError(t, err)
	}()

	log.Println("test log")

	f2, err := SetErrorLogger("invalid//name")
	require.Error(t, err)
	require.Nil(t, f2)
}
