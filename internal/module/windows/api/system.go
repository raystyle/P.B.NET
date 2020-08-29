package api

import (
	"golang.org/x/sys/windows"
)

// CloseHandle is used to close handle.
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
