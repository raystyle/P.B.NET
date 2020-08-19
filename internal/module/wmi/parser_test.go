// +build windows

package wmi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildWQL(t *testing.T) {
	wql := BuildWQL(testWin32Process{}, "Win32_Process")
	require.Equal(t, "select Name, ProcessId from Win32_Process", wql)
}
