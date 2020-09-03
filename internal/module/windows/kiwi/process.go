package kiwi

import (
	"time"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/logger"
	"project/internal/module/windows/api"
)

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

type basicModuleInfo struct {
	name      string
	address   uintptr
	size      int
	timestamp int64
}

func (kiwi *Kiwi) getVeryBasicModuleInfo(pHandle windows.Handle) ([]*basicModuleInfo, error) {
	const paddingSize = 256 + 512
	// read PEB base address
	infoClass := api.InfoClassProcessBasicInformation
	var pbi api.ProcessBasicInformation
	size := unsafe.Sizeof(pbi)
	_, err := api.NTQueryInformationProcess(pHandle, infoClass, (*byte)(unsafe.Pointer(&pbi)), size)
	if err != nil {
		return nil, err
	}
	kiwi.logf(logger.Debug, "PEB base address is 0x%X", pbi.PEBBaseAddress)
	// read PEB
	var peb struct {
		api.PEB
		padding [paddingSize]byte
	}
	size = unsafe.Sizeof(peb.PEB) + uintptr(256+kiwi.rand.Int(512))
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
	begin := loaderData.InMemoryOrderModuleVector.FLink - offset
	end := peb.LoaderData + unsafe.Offsetof(api.PEBLDRData{}.InLoadOrderModuleVector)
	kiwi.logf(logger.Debug, "read loader data table entry, begin: 0x%X, end: 0x%X", begin, end)
	var modules []*basicModuleInfo
	var entry struct {
		api.LDRDataTableEntry
		padding [paddingSize]byte
	}
	// prevent dead loop
	ticker := time.NewTicker(3 * time.Millisecond)
	defer ticker.Stop()
	for address := begin; address < end; address = entry.InMemoryOrderLinks.FLink - offset {
		// prevent dead loop
		select {
		case <-ticker.C:
		case <-kiwi.context.Done():
			return nil, kiwi.context.Err()
		}
		// read entry
		size = unsafe.Sizeof(entry.LDRDataTableEntry) + uintptr(256+kiwi.rand.Int(512))
		_, err = api.ReadProcessMemory(pHandle, address, (*byte)(unsafe.Pointer(&entry)), size)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to read loader data table entry")
		}
		// read base dll name
		name, err := api.ReadLSAUnicodeString(pHandle, &entry.BaseDLLName)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to read base dll name")
		}
		// add module
		modules = append(modules, &basicModuleInfo{
			name:    name,
			address: entry.DLLBase,
			size:    int(entry.SizeOfImage),
		})
	}
	kiwi.log(logger.Debug, "loaded module count:", len(modules))
	return modules, nil
}