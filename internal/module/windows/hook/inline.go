// +build windows

package hook

import (
	"bytes"
	"fmt"
	"reflect"
	"runtime"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/arch/x86/x86asm"
	"golang.org/x/sys/windows"
)

const (
	shortJumperSize = 1 + 4
	hookJumperSize
)

// PatchGuard contain information about hooked function.
type PatchGuard struct {
	Original *windows.Proc
}

// Patch is used to patch the target function.
func (pg *PatchGuard) Patch() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	return pg.patch()
}

func (pg *PatchGuard) patch() error {
	return nil
}

func (pg *PatchGuard) UnPatch() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	return pg.unPatch()
}

func (pg *PatchGuard) unPatch() error {
	return nil
}

func (pg *PatchGuard) Close() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	_ = pg.unPatch()
	return nil
}

// NewInlineHookByName is used to create a hook about function by DLL name and Proc name.
func NewInlineHookByName(dll, proc string, system bool, hookFn interface{}) (*PatchGuard, error) {
	// load DLL
	var lazyDLL *windows.LazyDLL
	if system {
		lazyDLL = windows.NewLazySystemDLL(dll)
	} else {
		lazyDLL = windows.NewLazyDLL(dll)
	}
	err := lazyDLL.Load()
	if err != nil {
		return nil, err
	}
	// find proc
	lazyProc := lazyDLL.NewProc(proc)
	err = lazyProc.Find()
	if err != nil {
		return nil, err
	}
	p := &windows.Proc{
		Name: dll,
	}
	p.Dll = &windows.DLL{
		Name:   dll,
		Handle: windows.Handle(lazyDLL.Handle()),
	}
	// set private structure field "addr"
	*(*uintptr)(unsafe.Pointer(
		reflect.ValueOf(p).Elem().FieldByName("addr").UnsafeAddr()),
	) = lazyProc.Addr()
	return NewInlineHook(p, hookFn)
}

// NewInlineHook is used to create a hook about function, usually hook a syscall.
func NewInlineHook(target *windows.Proc, hookFn interface{}) (*PatchGuard, error) {
	// select architecture
	arch, err := newArch()
	if err != nil {
		return nil, err
	}
	// read function address
	targetAddr := target.Addr()
	hookFnAddr := windows.NewCallback(hookFn)

	fmt.Printf("0x%X,0x%X\n", targetAddr, hookFnAddr)

	// inaccurate but sufficient
	// Prefix = 4, OpCode = 3, ModRM = 1,
	// sib = 1, displacement = 4, immediate = 4
	const maxCodeLen = 2 * (4 + 3 + 1 + 1 + 4 + 4)
	originFunc := unsafeReadMemory(targetAddr, maxCodeLen)
	// get instructions
	insts, err := disassemble(originFunc, arch.DisassembleMode())
	if err != nil {
		return nil, errors.Wrap(err, "failed to disassemble")
	}
	// get patch size that need fix
	patchSize, instNum, err := getASMPatchSizeAndInstNum(insts)
	if err != nil {
		return nil, err
	}

	createHookJumper(targetAddr)

	fmt.Println(patchSize, instNum)

	fmt.Println(insts)

	return nil, nil
}

func disassemble(src []byte, mode int) ([]*x86asm.Inst, error) {
	var r []*x86asm.Inst
	for len(src) > 0 {
		inst, err := x86asm.Decode(src, mode)
		if err != nil {
			return nil, err
		}
		r = append(r, &inst)
		src = src[inst.Len:]
	}
	return r, nil
}

// if appear too short function, we can improve it that add jumper to it,
// and use behind address to storage old code.
func getASMPatchSizeAndInstNum(insts []*x86asm.Inst) (int, int, error) {
	var (
		l int
		n int
	)
	for i := 0; i < len(insts); i++ {
		l += insts[i].Len
		n += 1
		if l >= shortJumperSize {
			return l, n, nil
		}
	}
	return 0, 0, errors.New("unable to insert jumper to this function")
}

// createHookJumper will create a far jumper to our hook function.
// only x64 need it.
func createHookJumper(target uintptr) (*memory, error) {
	const maxRange = 1024 * 1024 // only 1 GB
	begin := target - 14         // TODO: set random address
	var addr uintptr
	// first try to search low address
	for i := uintptr(0); i < maxRange; i++ {
		addr = begin - i

		if isAllInt3(addr) {

			fmt.Printf("0x%X\n", addr)

			return nil, nil

		}

	}

	// if not exist, search high address

	//

	return nil, nil
}

var allInt3 = bytes.Repeat([]byte{0xCC}, 14)

func isAllInt3(addr uintptr) bool {
	return bytes.Equal(unsafeReadMemory(addr, 14), allInt3)
}
