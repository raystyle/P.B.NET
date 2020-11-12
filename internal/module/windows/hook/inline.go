// +build windows

package hook

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"runtime"
	"unsafe"

	"github.com/davecgh/go-spew/spew"
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
		return nil, errors.WithMessage(err, "failed to disassemble original function")
	}
	// get patch size that need fix for trampoline function
	patchSize, instNum, err := getASMPatchSizeAndInstNum(insts)
	if err != nil {
		return nil, err
	}
	rebuilt, newInsts, err := rebuildInstruction(originalFunc[:patchSize], insts[:instNum])
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
	hookJumperData := arch.NewFarJumpASM(hookJumperMem.Addr, hookFnAddr)

	err = hookJumperMem.Write(hookJumperData)
	if err != nil {
		return nil, err
	}

	fmt.Printf("hook jumper: 0x%X\n", hookJumperMem.Addr)

	// create patch for jump to hook jumper
	patchMem := newMemory(targetAddr, nearJumperSize)

	patchData := newNearJumpASM(targetAddr, hookJumperMem.Addr)
	err = patchMem.Write(patchData)
	if err != nil {
		return nil, err
	}

	trampolineMem, err := searchMemory(targetAddr, patchSize+nearJumperSize, false, mbi)
	if err != nil {
		return nil, err
	}
	// create trampoline function for call original function
	trampoline := relocateInstruction(int(trampolineMem.Addr)-int(targetAddr), rebuilt, newInsts)
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
			return nil, errors.WithStack(err)
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
	mem, err := searchMemoryAt(begin, size, !low, mbi)
	if err == nil {
		return mem, nil
	}
	return searchMemoryAt(begin, size, low, mbi)
}

func searchMemoryAt(begin uintptr, size int, add bool, mbi *api.MemoryBasicInformation) (*memory, error) {
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

// rebuildInstruction is used to rebuild instruction for replace jmp short to jmp ...
// [Warning]: it is only partially done.
func rebuildInstruction(src []byte, insts []*x86asm.Inst) ([]byte, []*x86asm.Inst, error) {
	rebuilt := make([]byte, 0, len(src))
	for i := 0; i < len(insts); i++ {
		switch insts[i].Op {
		case x86asm.JMP:
			switch src[0] {
			case 0xEB: // replace to near jump
				inst := make([]byte, 5)
				inst[0] = 0xE9
				inst[1] = src[1]
				rebuilt = append(rebuilt, inst...)
			default:
				rebuilt = append(rebuilt, src[:insts[i].Len]...)
			}
		default:
			rebuilt = append(rebuilt, src[:insts[i].Len]...)
		}
		src = src[insts[i].Len:]
	}
	newInsts, err := disassemble(rebuilt, insts[0].Mode)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "failed to disassemble rebuilt instruction")
	}
	return rebuilt, newInsts, nil
}

// relocateInstruction is used to relocate instruction like jmp, call...
// [Warning]: it is only partially done.
func relocateInstruction(offset int, src []byte, insts []*x86asm.Inst) []byte {
	codeCp := make([]byte, len(src))
	copy(codeCp, src)
	src = codeCp
	relocated := make([]byte, 0, len(src))
	for i := 0; i < len(insts); i++ {

		switch insts[i].Op {
		case x86asm.CALL:

			spew.Config.DisableMethods = true
			spew.Dump(insts[i])

			switch src[0] {
			case 0xFF:
				switch src[1] {
				case 0x15:
					mem := insts[i].Args[0].(x86asm.Mem)
					binary.LittleEndian.PutUint32(src[2:], uint32(mem.Disp)-uint32(offset))
				}
			}
		}
		relocated = append(relocated, src[:insts[i].Len]...)
		src = src[insts[i].Len:]
	}
	return relocated
}
