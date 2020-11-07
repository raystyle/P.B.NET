// +build windows

package api

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procGetSystemInfo       = modKernel32.NewProc("GetSystemInfo")
	procGetNativeSystemInfo = modKernel32.NewProc("GetNativeSystemInfo")
)

// CloseHandle is used to close handle it will return error.
func CloseHandle(handle windows.Handle) {
	_ = windows.CloseHandle(handle)
}

// GetVersionNumber is used to get NT version number.
func GetVersionNumber() (major, minor, build uint32) {
	return windows.RtlGetNtVersionNumbers()
}

// VersionInfo contains information about Windows version.
type VersionInfo struct {
	Major            uint32
	Minor            uint32
	Build            uint32
	PlatformID       uint32
	CSDVersion       string
	ServicePackMajor uint16
	ServicePackMinor uint16
	SuiteMask        uint16
	ProductType      byte
}

// GetVersion is used ti get NT version information.
func GetVersion() *VersionInfo {
	ver := windows.RtlGetVersion()
	return &VersionInfo{
		Major:            ver.MajorVersion,
		Minor:            ver.MinorVersion,
		Build:            ver.BuildNumber,
		PlatformID:       ver.PlatformId,
		CSDVersion:       windows.UTF16ToString(ver.CsdVersion[:]),
		ServicePackMajor: ver.ServicePackMajor,
		ServicePackMinor: ver.ServicePackMinor,
		SuiteMask:        ver.SuiteMask,
		ProductType:      ver.ProductType,
	}
}

// about processor architecture.
const (
	ProcessorArchitectureAMD64   uint16 = 9      // x64 (AMD or Intel)
	ProcessorArchitectureARM     uint16 = 5      // ARM
	ProcessorArchitectureARM64   uint16 = 12     // ARM64
	ProcessorArchitectureIA64    uint16 = 6      // Intel Itanium-based
	ProcessorArchitectureIntel   uint16 = 0      // x86
	ProcessorArchitectureUnknown uint16 = 0xFFFF // Unknown architecture
)

var processorArchitectures = map[uint16]string{
	ProcessorArchitectureAMD64:   "x64",
	ProcessorArchitectureARM:     "arm",
	ProcessorArchitectureARM64:   "arm64",
	ProcessorArchitectureIA64:    "ia64",
	ProcessorArchitectureIntel:   "x86",
	ProcessorArchitectureUnknown: "unknown",
}

// GetProcessorArchitecture is used to convert architecture to string.
func GetProcessorArchitecture(arch uint16) string {
	str, ok := processorArchitectures[arch]
	if !ok {
		return "unknown"
	}
	return str
}

// SystemInfo contains system information.
type SystemInfo struct {
	ProcessorArchitecture     uint16
	reserved                  uint16
	oemID                     uint32 // obsolete
	PageSize                  uint32
	MinimumApplicationAddress uint32
	MaximumApplicationAddress uint32
	ActiveProcessorMask       uintptr // *uint32
	NumberOfProcessors        uint32
	ProcessorType             uint32
	AllocationGranularity     uint32
	ProcessorLevel            uint16
	ProcessorRevision         uint16
}

// GetSystemInfo is used to get system information. // #nosec
func GetSystemInfo() *SystemInfo {
	systemInfo := new(SystemInfo)
	_, _, _ = procGetSystemInfo.Call(uintptr(unsafe.Pointer(systemInfo)))
	return systemInfo
}

// GetNativeSystemInfo is used to get native system information. // #nosec
func GetNativeSystemInfo() *SystemInfo {
	systemInfo := new(SystemInfo)
	_, _, _ = procGetNativeSystemInfo.Call(uintptr(unsafe.Pointer(systemInfo)))
	return systemInfo
}

// IsSystem64Bit is used to check system is x64.
func IsSystem64Bit(unknown bool) bool {
	systemInfo := GetNativeSystemInfo()
	switch systemInfo.ProcessorArchitecture {
	case ProcessorArchitectureARM, ProcessorArchitectureIntel:
		return false
	case ProcessorArchitectureAMD64, ProcessorArchitectureARM64, ProcessorArchitectureIA64:
		return true
	default:
		return unknown
	}
}

// IsSystem32Bit is used to check system is x86.
func IsSystem32Bit(unknown bool) bool {
	systemInfo := GetNativeSystemInfo()
	switch systemInfo.ProcessorArchitecture {
	case ProcessorArchitectureARM, ProcessorArchitectureIntel:
		return true
	case ProcessorArchitectureAMD64, ProcessorArchitectureARM64, ProcessorArchitectureIA64:
		return false
	default:
		return unknown
	}
}
