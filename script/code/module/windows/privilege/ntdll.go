package privilege

import (
	"fmt"
	"strings"
)

func generateRtlEnableDisable(privilege, comment string) {
	// <p0> without "SE"
	const tpl = `
// RtlEnable<p0> is used to enable <c> privilege that call RtlAdjustPrivilege.
func RtlEnable<p0>() (bool, error) {
	return RtlAdjustPrivilege(<p>, true, false)
}

// RtlDisable<p0> is used to disable <c> privilege that call RtlAdjustPrivilege.
func RtlDisable<p0>() (bool, error) {
	return RtlAdjustPrivilege(<p>, false, false)
}
`
	src := strings.ReplaceAll(tpl, "<p0>", privilege[2:])
	src = strings.ReplaceAll(src, "<c>", comment)
	src = strings.ReplaceAll(src, "<p>", privilege)
	fmt.Print(src)
}

func generateTestRtlEnableDisable(privilege string) {
	const tpl = `
func TestRtlEnable<p0>(t *testing.T) {
	if !testIsElevated() {
		return
	}
	previous, err := RtlEnable<p0>()
	require.NoError(t, err)
	
	testRestorePrivilege(t, <p>, previous, true)
}

func TestRtlDisable<p0>(t *testing.T) {
	if !testIsElevated() {
		return
	}
	first, err := RtlEnable<p0>()
	require.NoError(t, err)

	previous, err := RtlDisable<p0>()
	require.NoError(t, err)
	require.True(t, previous)
	
	testRestorePrivilege(t, <p>, first, false)
}
`
	src := strings.ReplaceAll(tpl, "<p0>", privilege[2:])
	src = strings.ReplaceAll(src, "<p>", privilege)
	fmt.Print(src)
}
