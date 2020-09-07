// +build windows

package privilege

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"

	"project/internal/patch/monkey"
)

func TestAdjustPrivilege(t *testing.T) {
	t.Run("failed to OpenProcessToken", func(t *testing.T) {
		patch := func(windows.Handle, uint32, *windows.Token) error {
			return monkey.Error
		}
		pg := monkey.Patch(windows.OpenProcessToken, patch)
		defer pg.Unpatch()

		err := EnablePrivilege("")
		monkey.IsExistMonkeyError(t, err)
	})

	t.Run("invalid privilege name", func(t *testing.T) {
		err := EnablePrivilege("foo")
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("failed to AdjustTokenPrivileges", func(t *testing.T) {
		patch := func(
			windows.Token, bool, *windows.Tokenprivileges,
			uint32, *windows.Tokenprivileges, *uint32,
		) error {
			return monkey.Error
		}
		pg := monkey.Patch(windows.AdjustTokenPrivileges, patch)
		defer pg.Unpatch()

		err := EnablePrivilege(SeDebug)
		monkey.IsExistMonkeyError(t, err)
	})
}

func TestEnableDebug(t *testing.T) {
	err := EnableDebug()
	require.NoError(t, err)
}

func TestEnableShutdown(t *testing.T) {
	err := EnableShutdown()
	require.NoError(t, err)
}
