package system

import (
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
