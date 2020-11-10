// +build windows

package hook

import (
	"fmt"
	"reflect"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/arch/x86/x86asm"
	"golang.org/x/sys/windows"
)

// Guard contain information about hooked function.
type Guard struct {
	Original *windows.Proc
}

// NewInlineHookByName is used to create a hook about function by DLL name and Proc name.
func NewInlineHookByName(dll, proc string, system bool, hookFn interface{}) (*Guard, error) {
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
func NewInlineHook(target *windows.Proc, hookFn interface{}) (*Guard, error) {
	// select architecture
	arch, err := newArch()
	if err != nil {
		return nil, err
	}
	// read function address
	targetAddr := target.Addr()
	hookFnAddr := windows.NewCallback(hookFn)
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
	fmt.Println(insts)

	fmt.Println(targetAddr, hookFnAddr)

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
