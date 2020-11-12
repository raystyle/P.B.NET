// +build windows

package hook

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"runtime"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/arch/x86/x86asm"
	"golang.org/x/sys/windows"

	"project/internal/module/windows/api"
	"project/internal/random"
)

// [patch] is a near jump to [hook jumper].
// [hook jumper] is a far jump to our hook function.
// [trampoline] is a part of code about original function and
// add a near jump to the remaining original function.

// <security> not use FlushInstructionCache for bypass AV.

// PatchGuard contain information about hooked function.
type PatchGuard struct {
	Original *windows.Proc

	originalData []byte // contain origin data before hook
	patchData    []byte

	hookJumperMem  *memory
	hookJumperData []byte

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
	// Prefix = 4, OpCode = 3, ModRM = 1, sib = 1, displacement = 4, immediate = 4
	const maxInstLen = 2 * (4 + 3 + 1 + 1 + 4 + 4)
	originalFunc, err := readMemory(targetAddr, maxInstLen)
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
	// query memory information about target procedure address for
	// search writeable memory about hook jumper and trampoline.
	mbi, err := api.VirtualQuery(targetAddr)
	if err != nil {
		return nil, err
	}
	hookJumperMem, err := createHookJumper(arch, targetAddr, mbi)
	if err != nil {
		return nil, err
	}
	hookJumperData := arch.NewFarJumpASM(0, hookFnAddr)

	err = hookJumperMem.Write(hookJumperData)
	if err != nil {
		return nil, err
	}
	// create patch for jump to hook jumper
	shortJumper := newNearJumpASM(targetAddr, hookJumperMem.Addr)
	mem2 := newMemory(targetAddr, nearJumperSize)
	err = mem2.Write(shortJumper)
	if err != nil {
		return nil, err
	}

	// copy part of instruction about original function

	trampolineMem, err := searchMemory(targetAddr, patchSize+nearJumperSize, false, mbi)
	if err != nil {
		return nil, err
	}
	// create trampoline function for call original function
	trampoline := relocateInstruction(
		int(trampolineMem.Addr)-int(targetAddr), originalFunc[:patchSize], insts[:instNum])
	trampoline = append(trampoline, newNearJumpASM(
		trampolineMem.Addr+uintptr(len(trampoline)), targetAddr+uintptr(patchSize))...)

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
	var insts []*x86asm.Inst
	for len(src) > 0 {
		inst, err := x86asm.Decode(src, mode)
		if err != nil {
			return nil, err
		}
		insts = append(insts, &inst)
		src = src[inst.Len:]
	}
	return insts, nil
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

// createHookJumper will create a far jumper to our hook function.
func createHookJumper(arch arch, target uintptr, mbi *api.MemoryBasicInformation) (*memory, error) {
	return searchMemory(target, arch.FarJumpSize(), true, mbi)
}

// searchMemory is used to search memory for write hook jumper and trampoline.
// if low is true search low address first.
func searchMemory(begin uintptr, size int, low bool, mbi *api.MemoryBasicInformation) (*memory, error) {
	fmt.Printf("begin: 0x%X\n", begin)
	mem, err := searchMemoryAt(begin, size, !low, mbi)
	if err == nil {
		return mem, nil
	}
	return searchMemoryAt(begin, size, low, mbi)
}

func searchMemoryAt(begin uintptr, size int, add bool, mbi *api.MemoryBasicInformation) (*memory, error) {
	minBoundary := mbi.AllocationBase
	maxBoundary := mbi.AllocationBase + mbi.RegionSize
	fmt.Printf("min: 0x%X\n", minBoundary)
	fmt.Printf("max: 0x%X\n", maxBoundary)
	// set random begin address
	var maxRange uintptr
	rand := random.NewRand()
	if add {
		boundary := mbi.AllocationBase + mbi.RegionSize
		begin += 16 + uintptr(rand.Int(int(boundary-begin)/10))
		maxRange = boundary - begin - uintptr(size)
	} else {
		boundary := mbi.AllocationBase
		begin -= uintptr(size) + uintptr(rand.Int(int(begin-boundary)/10))
		maxRange = begin - boundary
	}
	var addr uintptr
	for i := uintptr(0); i < maxRange; i++ {
		if add {
			addr = begin + i
		} else {
			addr = begin - i
		}
		mem, err := readMemory(addr, size)
		if err != nil {
			return nil, err
		}
		// check memory is all int3 code
		if bytes.Equal(mem, bytes.Repeat([]byte{0xCC}, size)) {
			return newMemory(addr, size), nil
		}
	}
	return nil, errors.New("failed to search writeable memory")
}

// relocateInstruction is used to relocate instruction like jmp, call.
// [Warning]: it is only partially done.
func relocateInstruction(offset int, code []byte, insts []*x86asm.Inst) []byte {
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
					mem := insts[i].Args[0].(x86asm.Mem)
					binary.LittleEndian.PutUint32(code[2:], uint32(mem.Disp)-uint32(offset))
				}
			}
		}
		relocated = append(relocated, code[:insts[i].Len]...)
		code = code[insts[i].Len:]
	}
	return relocated
}
