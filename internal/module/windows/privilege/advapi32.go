// +build windows

package privilege

import (
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
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
		return errors.Wrapf(err, "failed to lookup %s", name)
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

// EnableDebug is used to enable the debug privilege.
func EnableDebug() error {
	return EnablePrivilege("SeDebugPrivilege")
}
