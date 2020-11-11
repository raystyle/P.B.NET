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
	hookJumperSize  = 14
)

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
	// select architecture
	arch := newArch()
	// read function address
	targetAddr := target.Addr()
	hookFnAddr := windows.NewCallback(hookFn)

	fmt.Printf("0x%X,0x%X\n", targetAddr, hookFnAddr)

	// inaccurate but sufficient
	// Prefix = 4, OpCode = 3, ModRM = 1,
	// sib = 1, displacement = 4, immediate = 4
	const maxCodeLen = 2 * (4 + 3 + 1 + 1 + 4 + 4)
	originalFunc, err := unsafeReadMemory(targetAddr, maxCodeLen)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read memory about original function")
	}
	// get instructions
	insts, err := disassemble(originalFunc, arch.DisassembleMode())
	if err != nil {
		return nil, errors.Wrap(err, "failed to disassemble")
	}
	// get patch size that need fix
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
	shortJumper := createShortJumper(targetAddr, memory.Addr)

	mem2 := newMemory(targetAddr, shortJumperSize)
	err = mem2.Write(shortJumper)
	if err != nil {
		return nil, err
	}

	// create original function

	originalFn := make([]byte, patchSize+14) // far jumper size
	// relocate address about some instruction
	relocatedCode := relocateInstruction(originalFunc[:patchSize], insts[:instNum])
	// copy part of instruction about original function
	copy(originalFn, relocatedCode)
	copy(originalFn[patchSize:], arch.NewFarJumpASM(0, targetAddr+uintptr(patchSize)))

	// //
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&originalFn))
	old := new(uint32)

	fmt.Printf("0x%X\n", sh.Data)
	fmt.Println("sh", &originalFn[0])

	err = windows.VirtualProtect(sh.Data, uintptr(sh.Len), windows.PAGE_EXECUTE_READWRITE, old)
	if err != nil {
		return nil, err
	}
	proc := &windows.Proc{
		Dll:  target.Dll,
		Name: target.Name,
	}
	*(*uintptr)(unsafe.Pointer(
		reflect.ValueOf(proc).Elem().FieldByName("addr").UnsafeAddr()),
	) = sh.Data

	pg := PatchGuard{
		Original: proc,
		fnData:   originalFn,
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
		l int
		n int
	)
	for i := 0; i < len(insts); i++ {
		l += insts[i].Len
		n++
		if l >= shortJumperSize {
			return l, n, nil
		}
	}
	return 0, 0, errors.New("unable to insert jumper to this function")
}

var allInt3 = bytes.Repeat([]byte{0xCC}, 14)

// createHookJumper will create a far jumper to our hook function.
func createHookJumper(target uintptr) (*memory, error) {
	const maxRange = 1024 * 1024 // only 1 GB
	var addr uintptr
	// first try to search low address
	begin := target - hookJumperSize - 0 // TODO: set random address
	for i := uintptr(0); i < maxRange; i++ {
		addr = begin - i
		// check is all int3 code
		if bytes.Equal(unsafeReadMemory(addr, hookJumperSize), allInt3) {
			// find the address that can write hook jumper
			return newMemory(addr, hookJumperSize), nil
		}
	}
	// if not exist, search high address
	begin = target + 1024 // TODO: set random address
	for i := uintptr(0); i < maxRange; i++ {
		addr = begin + i
		// check is all int3 code
		if bytes.Equal(unsafeReadMemory(addr, hookJumperSize), allInt3) {
			// find the address that can write hook jumper
			return newMemory(addr, hookJumperSize), nil
		}
	}
	return nil, errors.New("no memory for create hook jumper")
}

// createShortJumper is used to create a jumper to the hook jumper.
func createShortJumper(from, to uintptr) []byte {
	asm := make([]byte, 5)
	asm[0] = 0xE9 // jmp rel32
	*(*int32)(unsafe.Pointer(&asm[1])) = int32(to) - int32(from) - int32(5)
	return asm
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
