package privilege

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestRtlAdjustPrivilege(t *testing.T) {
	_, err := RtlAdjustPrivilege(0, true, true)
	require.Error(t, err)
}

func testIsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func testResetPrivilege(t *testing.T, previous bool, id uint32) {
	if previous {
		_, err := RtlAdjustPrivilege(id, true, false)
		require.NoError(t, err)
	} else {
		_, err := RtlAdjustPrivilege(id, false, false)
		require.NoError(t, err)
	}
}

func TestRtlEnableDebug(t *testing.T) {
	if !testIsElevated() {
		return
	}
	previous, err := RtlEnableDebug()
	require.NoError(t, err)
	require.True(t, previous)

	testResetPrivilege(t, previous, SEDebug)
}

func TestRtlDisableDebug(t *testing.T) {
	if !testIsElevated() {
		return
	}
	first, err := RtlEnableDebug()
	require.NoError(t, err)
	require.True(t, first)

	previous, err := RtlDisableDebug()
	require.NoError(t, err)
	require.True(t, previous)

	testResetPrivilege(t, first, SEDebug)
}
