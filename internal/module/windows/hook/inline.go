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

	"project/internal/module/windows/api"
)

// [patch] is a near jump to [hook jumper].
// [hook jumper] is a far jump to our hook function.
// [trampoline] is a part of code about original function and
// add a near jump to the remaining original function.

// PatchGuard contain information about hooked function.
type PatchGuard struct {
	Original *windows.Proc

	fnData []byte
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

// UnPatch is used to unpatch the target function.
func (pg *PatchGuard) UnPatch() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	return pg.unPatch()
}

func (pg *PatchGuard) unPatch() error {
	return nil
}

// Close is used to unpatch the target function and release memory.
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
	arch := newArch()
	// read function address
	targetAddr := target.Addr()
	hookFnAddr := windows.NewCallback(hookFn)

	fmt.Printf("0x%X,0x%X\n", targetAddr, hookFnAddr)

	// inaccurate but sufficient
	// Prefix = 4, OpCode = 3, ModRM = 1,
	// sib = 1, displacement = 4, immediate = 4
	const maxInstLen = 2 * (4 + 3 + 1 + 1 + 4 + 4)
	originalFunc, err := unsafeReadMemory(targetAddr, maxInstLen)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read memory about original function")
	}
	// get instructions
	insts, err := disassemble(originalFunc, arch.DisassembleMode())
	if err != nil {
		return nil, errors.Wrap(err, "failed to disassemble")
	}
	// get patch size that need fix for trampoline function
	patchSize, instNum, err := getASMPatchSizeAndInstNum(insts)
	if err != nil {
		return nil, err
	}

	memory, err := createHookJumper(targetAddr)
	if err != nil {
		return nil, err
	}
	err = memory.Write(arch.NewFarJumpASM(0, hookFnAddr))
	if err != nil {
		return nil, err
	}
	// create jumper for hook jumper
	shortJumper := newNearJumpASM(targetAddr, memory.Addr)

	mem2 := newMemory(targetAddr, nearJumperSize)
	err = mem2.Write(shortJumper)
	if err != nil {
		return nil, err
	}

	// create trampoline function for call original function
	trampoline := relocateInstruction(originalFunc[:patchSize], insts[:instNum])
	// copy part of instruction about original function

	trampolineMem, err := searchWriteableMemory(targetAddr, len(trampoline)+nearJumperSize, false)
	if err != nil {
		return nil, err
	}
	trampoline = append(trampoline, newNearJumpASM(trampolineMem.Addr+uintptr(len(trampoline)), targetAddr+uintptr(patchSize))...)

	err = trampolineMem.Write(trampoline)
	if err != nil {
		return nil, err
	}

	// copy(trampoline[patchSize:], arch.NewFarJumpASM(0, targetAddr+uintptr(patchSize)))

	// //
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&trampoline))
	old := new(uint32)

	fmt.Printf("0x%X\n", sh.Data)
	fmt.Println("sh", &trampoline[0])

	err = api.VirtualProtect(sh.Data, uintptr(sh.Len), windows.PAGE_EXECUTE_READWRITE, old)
	if err != nil {
		return nil, err
	}
	proc := &windows.Proc{
		Dll:  target.Dll,
		Name: target.Name,
	}
	*(*uintptr)(unsafe.Pointer(
		reflect.ValueOf(proc).Elem().FieldByName("addr").UnsafeAddr()),
	) = trampolineMem.Addr

	pg := PatchGuard{
		Original: proc,
		fnData:   trampoline,
	}

	fmt.Println(patchSize, instNum)

	// fmt.Println(insts)

	return &pg, nil
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
		size int
		num  int
	)
	for i := 0; i < len(insts); i++ {
		size += insts[i].Len
		num++
		if size >= nearJumperSize {
			return size, num, nil
		}
	}
	return 0, 0, errors.New("unable to insert near jmp to this function")
}

// searchWriteableMemory is used to search memory for write hook jumper and trampoline.
func searchWriteableMemory(begin uintptr, size int, lowFirst bool) (*memory, error) {
	mem, err := searchMemoryAt(begin, size, !lowFirst)
	if err == nil {
		return mem, nil
	}
	return searchMemoryAt(begin, size, lowFirst)
}

func searchMemoryAt(begin uintptr, size int, add bool) (*memory, error) {
	const maxRange = 32 * 1024 * 1024 // only 32MB
	if add {
		// TODO: set random address
		// rand := random.NewRand()
		// begin += 512 + uintptr(rand.Int(4096))
		begin += 5
	} else {
		begin -= uintptr(size)
	}
	var addr uintptr
	for i := uintptr(0); i < maxRange; i++ {
		if add {
			addr = begin + i
		} else {
			addr = begin - i
		}
		mem, err := unsafeReadMemory(addr, size)
		if err != nil {
			continue
		}
		// check memory is all int3 code
		if bytes.Equal(mem, bytes.Repeat([]byte{0xCC}, size)) {
			return newMemory(addr, size), nil
		}
	}
	return nil, errors.New("failed to search writeable memory")
}

// createHookJumper will create a far jumper to our hook function.
func createHookJumper(target uintptr) (*memory, error) {
	return searchWriteableMemory(target, 14, true)
}

// relocateInstruction is used to relocate instruction like jmp, call.
func relocateInstruction(code []byte, insts []*x86asm.Inst) []byte {
	codeCp := make([]byte, len(code))
	copy(codeCp, code)
	code = codeCp
	relocated := make([]byte, 0, len(code))

	for i := 0; i < len(insts); i++ {
		switch insts[i].Op {
		case x86asm.CALL:
			switch code[0] {
			case 0xFF:
				switch code[1] {
				case 0x15:
					// change address

					mem := insts[i].Args[0].(x86asm.Mem)
					fmt.Println(mem.Disp)

				}
			}

		}
		relocated = append(relocated, code[:insts[i].Len]...)
		code = code[insts[i].Len:]
	}
	return relocated
}
