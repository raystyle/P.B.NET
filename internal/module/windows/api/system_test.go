package api

import (
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"
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
