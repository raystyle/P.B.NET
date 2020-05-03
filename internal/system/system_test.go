package system

import (
	"log"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestWriteFile(t *testing.T) {
	_ = os.Mkdir("testdata", 0750)

	const name = "wf.dat"
	testdata := testsuite.Bytes()

	t.Run("ok", func(t *testing.T) {
		defer func() {
			err := os.Remove(name)
			require.NoError(t, err)
		}()
		err := WriteFile(name, testdata)
		require.NoError(t, err)
	})

	t.Run("invalid path", func(t *testing.T) {
		err := WriteFile("invalid//name", testdata)
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
	patch := func() (string, error) {
		return "", monkey.Error
	}
	pg := monkey.Patch(os.Executable, patch)
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
