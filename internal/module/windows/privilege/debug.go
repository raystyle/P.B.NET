// +build windows

package privilege

import (
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

// EnableDebugPrivilege is used to enable the debug privilege in the current running process.
func EnableDebugPrivilege() error {
	// get current process token
	handle := windows.CurrentProcess()
	var token windows.Token
	err := windows.OpenProcessToken(handle, syscall.TOKEN_ADJUST_PRIVILEGES|syscall.TOKEN_QUERY, &token)
	if err != nil {
		return errors.Wrap(err, "failed to open current process token")
	}
	// lookup debug privilege
	debug := new(windows.LUID)
	err = windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr("SeDebugPrivilege"), debug)
	if err != nil {
		return errors.Wrap(err, "failed to lookup debug privilege")
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
		return errors.Wrap(err, "failed to enable debug privilege to current process token")
	}
	return nil
}
