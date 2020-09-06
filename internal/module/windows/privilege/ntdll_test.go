package privilege

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func testIsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func TestRtlEnableDebug(t *testing.T) {
	if !testIsElevated() {
		return
	}
	previous, err := RtlEnableDebug()
	require.NoError(t, err)
	require.False(t, previous)
}
