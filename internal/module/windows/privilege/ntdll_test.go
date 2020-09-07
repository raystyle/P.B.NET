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

func testRestorePrivilege(t *testing.T, id uint32, previous, now bool) {
	if previous != now {
		_, err := RtlAdjustPrivilege(id, previous, false)
		require.NoError(t, err)
	}
}

func TestRtlEnableSecurity(t *testing.T) {
	if !testIsElevated() {
		return
	}
	previous, err := RtlEnableSecurity()
	require.NoError(t, err)
	require.False(t, previous)

	testRestorePrivilege(t, SESecurity, previous, true)
}

func TestRtlDisableSecurity(t *testing.T) {
	if !testIsElevated() {
		return
	}
	first, err := RtlEnableSecurity()
	require.NoError(t, err)
	require.False(t, first)

	previous, err := RtlDisableSecurity()
	require.NoError(t, err)
	require.True(t, previous)

	testRestorePrivilege(t, SESecurity, first, false)
}

func TestRtlEnableLoadDriver(t *testing.T) {
	if !testIsElevated() {
		return
	}
	previous, err := RtlEnableLoadDriver()
	require.NoError(t, err)
	require.False(t, previous)

	testRestorePrivilege(t, SELoadDriver, previous, true)
}

func TestRtlDisableLoadDriver(t *testing.T) {
	if !testIsElevated() {
		return
	}
	first, err := RtlEnableLoadDriver()
	require.NoError(t, err)
	require.False(t, first)

	previous, err := RtlDisableLoadDriver()
	require.NoError(t, err)
	require.True(t, previous)

	testRestorePrivilege(t, SELoadDriver, first, false)
}

func TestRtlEnableSystemTime(t *testing.T) {
	if !testIsElevated() {
		return
	}
	previous, err := RtlEnableSystemTime()
	require.NoError(t, err)
	require.False(t, previous)

	testRestorePrivilege(t, SESystemTime, previous, true)
}

func TestRtlDisableSystemTime(t *testing.T) {
	if !testIsElevated() {
		return
	}
	first, err := RtlEnableSystemTime()
	require.NoError(t, err)
	require.False(t, first)

	previous, err := RtlDisableSystemTime()
	require.NoError(t, err)
	require.True(t, previous)

	testRestorePrivilege(t, SESystemTime, first, false)
}

func TestRtlEnableSystemProf(t *testing.T) {
	if !testIsElevated() {
		return
	}
	previous, err := RtlEnableSystemProf()
	require.NoError(t, err)
	require.False(t, previous)

	testRestorePrivilege(t, SESystemProf, previous, true)
}

func TestRtlDisableSystemProf(t *testing.T) {
	if !testIsElevated() {
		return
	}
	first, err := RtlEnableSystemProf()
	require.NoError(t, err)
	require.False(t, first)

	previous, err := RtlDisableSystemProf()
	require.NoError(t, err)
	require.True(t, previous)

	testRestorePrivilege(t, SESystemProf, first, false)
}

func TestRtlEnableBackup(t *testing.T) {
	if !testIsElevated() {
		return
	}
	previous, err := RtlEnableBackup()
	require.NoError(t, err)
	require.False(t, previous)

	testRestorePrivilege(t, SEBackup, previous, true)
}

func TestRtlDisableBackup(t *testing.T) {
	if !testIsElevated() {
		return
	}
	first, err := RtlEnableBackup()
	require.NoError(t, err)
	require.False(t, first)

	previous, err := RtlDisableBackup()
	require.NoError(t, err)
	require.True(t, previous)

	testRestorePrivilege(t, SEBackup, first, false)
}

func TestRtlEnableShutdown(t *testing.T) {
	if !testIsElevated() {
		return
	}
	previous, err := RtlEnableShutdown()
	require.NoError(t, err)
	require.False(t, previous)

	testRestorePrivilege(t, SEShutdown, previous, true)
}

func TestRtlDisableShutdown(t *testing.T) {
	if !testIsElevated() {
		return
	}
	first, err := RtlEnableShutdown()
	require.NoError(t, err)
	require.False(t, first)

	previous, err := RtlDisableShutdown()
	require.NoError(t, err)
	require.True(t, previous)

	testRestorePrivilege(t, SEShutdown, first, false)
}

func TestRtlEnableDebug(t *testing.T) {
	if !testIsElevated() {
		return
	}
	previous, err := RtlEnableDebug()
	require.NoError(t, err)
	require.True(t, previous)

	testRestorePrivilege(t, SEDebug, previous, true)
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

	testRestorePrivilege(t, SEDebug, first, false)
}

func TestRtlEnableSystemEnv(t *testing.T) {
	if !testIsElevated() {
		return
	}
	previous, err := RtlEnableSystemEnv()
	require.NoError(t, err)
	require.False(t, previous)

	testRestorePrivilege(t, SESystemEnv, previous, true)
}

func TestRtlDisableSystemEnv(t *testing.T) {
	if !testIsElevated() {
		return
	}
	first, err := RtlEnableSystemEnv()
	require.NoError(t, err)
	require.False(t, first)

	previous, err := RtlDisableSystemEnv()
	require.NoError(t, err)
	require.True(t, previous)

	testRestorePrivilege(t, SESystemEnv, first, false)
}

func TestRtlEnableRemoteShutdown(t *testing.T) {
	if !testIsElevated() {
		return
	}
	previous, err := RtlEnableRemoteShutdown()
	require.NoError(t, err)
	require.False(t, previous)

	testRestorePrivilege(t, SERemoteShutdown, previous, true)
}

func TestRtlDisableRemoteShutdown(t *testing.T) {
	if !testIsElevated() {
		return
	}
	first, err := RtlEnableRemoteShutdown()
	require.NoError(t, err)
	require.False(t, first)

	previous, err := RtlDisableRemoteShutdown()
	require.NoError(t, err)
	require.True(t, previous)

	testRestorePrivilege(t, SERemoteShutdown, first, false)
}
