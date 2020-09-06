package privilege

import (
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/kuhl_m_privilege.h
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/kuhl_m_privilege.c

var (
	modNTDLL = windows.NewLazySystemDLL("ntdll.dll")

	procRtlAdjustPrivilege = modNTDLL.NewProc("RtlAdjustPrivilege")
)

const (
	seLoadDriver uint32 = 20
	seSystemTime uint32 = 20
	seBackup     uint32 = 20
	seShutdown   uint32 = 20
	seDebug      uint32 = 20
)

// RtlAdjustPrivilege is used to adjust privilege with procRtlAdjustPrivilege.
func RtlAdjustPrivilege(id uint32, enable, currentThread bool) (bool, error) {
	var p0 uint32
	if enable {
		p0 = 1
	}
	var p1 uint32
	if currentThread {
		p1 = 1
	}
	var previous bool
	ret, _, _ := procRtlAdjustPrivilege.Call(
		uintptr(id), uintptr(p0), uintptr(p1), uintptr(unsafe.Pointer(&previous)),
	)
	if ret != 0 {
		return false, errors.Errorf("failed to enable privilege: %d, error: 0x%08X", id, ret)
	}
	return previous, nil
}

// RtlEnableDebug is used to enable debug privilege that call RtlAdjustPrivilege.
func RtlEnableDebug() (bool, error) {
	return RtlAdjustPrivilege(seDebug, true, false)
}
