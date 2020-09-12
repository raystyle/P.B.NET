package api

import (
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
)

func TestGetVersionNumber(t *testing.T) {
	major, minor, build := GetVersionNumber()
	fmt.Println("major:", major, "minor:", minor, "build:", build)
}

func TestGetVersion(t *testing.T) {
	info := GetVersion()
	spew.Dump(info)
}

func TestGetProcessorArchitecture(t *testing.T) {
	arch := GetProcessorArchitecture(1)
	require.Equal(t, "unknown", arch)
}

func TestGetSystemInfo(t *testing.T) {
	systemInfo := GetSystemInfo()
	spew.Dump(systemInfo)

	arch := GetProcessorArchitecture(systemInfo.ProcessorArchitecture)
	fmt.Println(arch)
}

func TestGetNativeSystemInfo(t *testing.T) {
	systemInfo := GetNativeSystemInfo()
	spew.Dump(systemInfo)

	arch := GetProcessorArchitecture(systemInfo.ProcessorArchitecture)
	fmt.Println(arch)
}

func TestIsSystem64Bit(t *testing.T) {
	t.Run("common", func(t *testing.T) {
		b := IsSystem64Bit(false)
		fmt.Println("is 64 Bit:", b)
	})

	t.Run("32 bit", func(t *testing.T) {
		patch := func() *SystemInfo {
			return &SystemInfo{
				ProcessorArchitecture: ProcessorArchitectureIntel,
			}
		}
		pg := monkey.Patch(GetNativeSystemInfo, patch)
		defer pg.Unpatch()

		b := IsSystem64Bit(false)
		require.False(t, b)
	})

	t.Run("64 bit", func(t *testing.T) {
		patch := func() *SystemInfo {
			return &SystemInfo{
				ProcessorArchitecture: ProcessorArchitectureAMD64,
			}
		}
		pg := monkey.Patch(GetNativeSystemInfo, patch)
		defer pg.Unpatch()

		b := IsSystem64Bit(false)
		require.True(t, b)
	})

	t.Run("unknown", func(t *testing.T) {
		patch := func() *SystemInfo {
			return &SystemInfo{
				ProcessorArchitecture: ProcessorArchitectureUnknown,
			}
		}
		pg := monkey.Patch(GetNativeSystemInfo, patch)
		defer pg.Unpatch()

		b := IsSystem64Bit(false)
		require.False(t, b)
		b = IsSystem64Bit(true)
		require.True(t, b)
	})
}

func TestIsSystem32Bit(t *testing.T) {
	t.Run("common", func(t *testing.T) {
		b := IsSystem32Bit(false)
		fmt.Println("is 32 Bit:", b)
	})

	t.Run("32 bit", func(t *testing.T) {
		patch := func() *SystemInfo {
			return &SystemInfo{
				ProcessorArchitecture: ProcessorArchitectureIntel,
			}
		}
		pg := monkey.Patch(GetNativeSystemInfo, patch)
		defer pg.Unpatch()

		b := IsSystem32Bit(false)
		require.True(t, b)
	})

	t.Run("64 bit", func(t *testing.T) {
		patch := func() *SystemInfo {
			return &SystemInfo{
				ProcessorArchitecture: ProcessorArchitectureAMD64,
			}
		}
		pg := monkey.Patch(GetNativeSystemInfo, patch)
		defer pg.Unpatch()

		b := IsSystem32Bit(false)
		require.False(t, b)
	})

	t.Run("unknown", func(t *testing.T) {
		patch := func() *SystemInfo {
			return &SystemInfo{
				ProcessorArchitecture: ProcessorArchitectureUnknown,
			}
		}
		pg := monkey.Patch(GetNativeSystemInfo, patch)
		defer pg.Unpatch()

		b := IsSystem32Bit(false)
		require.False(t, b)
		b = IsSystem32Bit(true)
		require.True(t, b)
	})
}
