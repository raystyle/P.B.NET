// +build windows

package privilege

import (
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

// about privilege name
const (
	SeDebug    = "SeDebugPrivilege"
	SeShutdown = "SeShutdownPrivilege"
)

// EnablePrivilege is used to enable privilege with privilege name.
func EnablePrivilege(name string) error {
	// get current process token
	handle := windows.CurrentProcess()
	var token windows.Token
	err := windows.OpenProcessToken(handle, windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token)
	if err != nil {
		return errors.Wrap(err, "failed to open current process token")
	}
	// lookup debug privilege
	debug := new(windows.LUID)
	err = windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(name), debug)
	if err != nil {
		return errors.Wrapf(err, "failed to lookup %q", name)
	}
	// adjust token privilege
	privilege := windows.Tokenprivileges{
		PrivilegeCount: 1,
		Privileges: [1]windows.LUIDAndAttributes{{
			Luid:       *debug,
			Attributes: windows.SE_PRIVILEGE_ENABLED,
		}},
	}
	err = windows.AdjustTokenPrivileges(token, false, &privilege, 0, nil, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to enable %s with current process token", name)
	}
	return nil
}

// EnableDebug is used to enable debug privilege.
// If no permission to enable, it will not return error, if you want to
// get the error, use RtlEnableDebug().
func EnableDebug() error {
	return EnablePrivilege(SeDebug)
}

// EnableShutdown is used to enable shutdown privilege.
// If no permission to enable, it will not return error, if you want to
// get the error, use RtlEnableShutdown().
func EnableShutdown() error {
	return EnablePrivilege(SeShutdown)
}
