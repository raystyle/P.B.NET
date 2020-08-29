package api

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// reference:
//
// https://docs.microsoft.com/en-us/windows/win32/api/bcrypt
// https://docs.microsoft.com/en-us/windows/win32/seccng/cng-algorithm-identifiers

var (
	modBcrypt = windows.NewLazySystemDLL("bcrypt.dll")

	procBCryptOpenAlgorithmProvider  = modBcrypt.NewProc("BCryptOpenAlgorithmProvider")
	procBCryptCloseAlgorithmProvider = modBcrypt.NewProc("BCryptCloseAlgorithmProvider")
	procBCryptSetProperty            = modBcrypt.NewProc("BCryptSetProperty")
	procBCryptGetProperty            = modBcrypt.NewProc("BCryptGetProperty")
	procBCryptGenerateSymmetricKey   = modBcrypt.NewProc("BCryptGenerateSymmetricKey")
)

// BcryptHandle is a provider by call BCryptOpenAlgorithmProvider.
type BcryptHandle uintptr

// BCryptOpenAlgorithmProvider loads and initializes a CNG provider.
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
		return 0, newErrorf(name, err, "failed to open algorithm provider %q", algID)
	}
	return BcryptHandle(handle), nil
}

// BCryptCloseAlgorithmProvider is used to closes an algorithm provider.
func BCryptCloseAlgorithmProvider(handle BcryptHandle, flags uint32) error {
	const name = "BCryptCloseAlgorithmProvider"
	ret, _, err := procBCryptCloseAlgorithmProvider.Call(uintptr(handle), uintptr(flags))
	if ret != 0 {
		return newErrorf(name, err, "failed to close algorithm provider with handle 0x%X", handle)
	}
	return nil
}

// BCryptSetProperty is used to set the value of a named property for a CNG object.
func BCryptSetProperty(handle BcryptHandle, prop, input string, flags uint32) error {
	const name = "BCryptSetProperty"
	propPtr, err := windows.UTF16PtrFromString(prop)
	if err != nil {
		return newError(name, err, "failed to call UTF16PtrFromString")
	}
	inputPtr, err := windows.UTF16PtrFromString(input)
	if err != nil {
		return newError(name, err, "failed to call UTF16PtrFromString")
	}
	ret, _, err := procBCryptSetProperty.Call(
		uintptr(handle), uintptr(unsafe.Pointer(propPtr)),
		uintptr(unsafe.Pointer(inputPtr)), uintptr(uint32(len(input))),
		uintptr(flags))
	if ret != 0 {
		return newErrorf(name, err, "failed to set property %q", prop)
	}
	return nil
}

// BCryptGetProperty is used to retrieves the value of a named property for a CNG object.
func BCryptGetProperty() {

}

// BCryptGenerateSymmetricKey is used to creates a key object for use with a
// symmetrical key encryption algorithm from a supplied key.
func BCryptGenerateSymmetricKey(
	alg uintptr,
	key *uintptr,
	pbKeyObject *byte,
	cbKeyObject uint32,
	pbSecret *byte,
	cbSecret uint32,
	flags uint32,
) error {
	const name = "BCryptGenerateSymmetricKey"
	ret, _, err := procBCryptGenerateSymmetricKey.Call(
		alg,
		uintptr(unsafe.Pointer(key)),
		uintptr(unsafe.Pointer(pbKeyObject)), uintptr(cbKeyObject),
		uintptr(unsafe.Pointer(pbSecret)), uintptr(cbSecret),
		uintptr(flags),
	)
	if ret != 0 {
		return newError(name, err, "failed to generate symmetric key")
	}
	return nil
}
