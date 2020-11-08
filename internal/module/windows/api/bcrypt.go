// +build windows

package api

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// reference:
// https://docs.microsoft.com/en-us/windows/win32/api/bcrypt
// https://docs.microsoft.com/en-us/windows/win32/seccng/cng-algorithm-identifiers

// BcryptHandle is a provider by call BCryptOpenAlgorithmProvider.
type BcryptHandle uintptr

// BCryptOpenAlgorithmProvider loads and initializes a CNG provider. // #nosec
func BCryptOpenAlgorithmProvider(algID, impl string, flags uint32) (BcryptHandle, error) {
	const name = "BCryptOpenAlgorithmProvider"
	algIDPtr, err := windows.UTF16PtrFromString(algID)
	if err != nil {
		return 0, newError(name, err, "failed to call UTF16PtrFromString")
	}
	var implPtr *uint16
	if impl != "" {
		implPtr, err = windows.UTF16PtrFromString(impl)
		if err != nil {
			return 0, newError(name, err, "failed to call UTF16PtrFromString")
		}
	}
	var handle uintptr
	ret, _, err := procBCryptOpenAlgorithmProvider.Call(
		uintptr(unsafe.Pointer(&handle)), uintptr(unsafe.Pointer(algIDPtr)),
		uintptr(unsafe.Pointer(implPtr)), uintptr(flags),
	)
	if ret != 0 {
		return 0, newErrorf(name, err, "failed to open algorithm provider \"%s\"", algID)
	}
	return BcryptHandle(handle), nil
}

// BCryptCloseAlgorithmProvider is used to closes an algorithm provider. // #nosec
func BCryptCloseAlgorithmProvider(handle BcryptHandle, flags uint32) error {
	const name = "BCryptCloseAlgorithmProvider"
	ret, _, err := procBCryptCloseAlgorithmProvider.Call(uintptr(handle), uintptr(flags))
	if ret != 0 {
		return newErrorf(name, err, "failed to close algorithm provider with handle 0x%X", handle)
	}
	return nil
}

// BCryptSetProperty is used to set the value of a named property for a CNG object. // #nosec
func BCryptSetProperty(handle BcryptHandle, prop string, input *byte, size, flags uint32) error {
	const name = "BCryptSetProperty"
	propPtr, err := windows.UTF16PtrFromString(prop)
	if err != nil {
		return newError(name, err, "failed to call UTF16PtrFromString")
	}
	ret, _, err := procBCryptSetProperty.Call(
		uintptr(handle), uintptr(unsafe.Pointer(propPtr)),
		uintptr(unsafe.Pointer(input)), uintptr(size), uintptr(flags),
	)
	if ret != 0 {
		return newErrorf(name, err, "failed to set property \"%s\"", prop)
	}
	return nil
}

// BCryptGetProperty is used to retrieves the value of a named property for a CNG object. // #nosec
func BCryptGetProperty(handle BcryptHandle, prop string, output *byte, size, flags uint32) (uint32, error) {
	const name = "BCryptGetProperty"
	propPtr, err := windows.UTF16PtrFromString(prop)
	if err != nil {
		return 0, newError(name, err, "failed to call UTF16PtrFromString")
	}
	var result uint32
	ret, _, err := procBCryptGetProperty.Call(
		uintptr(handle), uintptr(unsafe.Pointer(propPtr)),
		uintptr(unsafe.Pointer(output)), uintptr(size),
		uintptr(unsafe.Pointer(&result)), uintptr(flags),
	)
	if ret != 0 {
		return 0, newErrorf(name, err, "failed to get property \"%s\"", prop)
	}
	return result, nil
}

// BcryptKey contains provider handle, bcrypt key handle and CNG object.
type BcryptKey struct {
	Provider BcryptHandle
	Handle   uintptr // output, key handle
	Object   []byte  // make slice for set size parameter
	Secret   []byte  // input parameter
	Flags    uint32  // input parameter
}

// Destroy is used to destroy bcrypt key and close provider handle.
func (bk *BcryptKey) Destroy() error {
	if bk.Handle != 0 {
		err := BCryptDestroyKey(bk.Handle)
		if err != nil {
			return err
		}
		bk.Handle = 0
	}
	if bk.Provider != 0 {
		err := BCryptCloseAlgorithmProvider(bk.Provider, bk.Flags)
		if err != nil {
			return err
		}
		bk.Provider = 0
	}
	return nil
}

// BCryptGenerateSymmetricKey is used to creates a key object for use with a symmetrical
// symmetrical key encryption algorithm from a supplied key. // #nosec
func BCryptGenerateSymmetricKey(bk *BcryptKey) error {
	const name = "BCryptGenerateSymmetricKey"
	ret, _, err := procBCryptGenerateSymmetricKey.Call(
		uintptr(bk.Provider), uintptr(unsafe.Pointer(&bk.Handle)),
		uintptr(unsafe.Pointer(&bk.Object[0])), uintptr(uint32(len(bk.Object))),
		uintptr(unsafe.Pointer(&bk.Secret[0])), uintptr(uint32(len(bk.Secret))),
		uintptr(bk.Flags),
	)
	if ret != 0 {
		return newError(name, err, "failed to generate symmetric key")
	}
	return nil
}

// BCryptDestroyKey is used to destroys a key. // #nosec
func BCryptDestroyKey(handle uintptr) error {
	const name = "BCryptDestroyKey"
	ret, _, err := procBCryptDestroyKey.Call(handle)
	if ret != 0 {
		return newError(name, err, "failed to destroy symmetric key")
	}
	return nil
}

// BCryptDecrypt is used to decrypt a block of data. // #nosec
func BCryptDecrypt(bk *BcryptKey, input []byte, paddingInfo uintptr, iv, output []byte) (uint32, error) {
	const name = "BCryptDecrypt"
	var result uint32
	ret, _, err := procBCryptDecrypt.Call(
		bk.Handle,
		uintptr(unsafe.Pointer(&input[0])), uintptr(uint32(len(input))), paddingInfo,
		uintptr(unsafe.Pointer(&iv[0])), uintptr(uint32(len(iv))),
		uintptr(unsafe.Pointer(&output[0])), uintptr(uint32(len(output))),
		uintptr(unsafe.Pointer(&result)), uintptr(bk.Flags),
	)
	if ret != 0 {
		return result, newError(name, err, "failed to decrypt block data")
	}
	return result, nil
}
