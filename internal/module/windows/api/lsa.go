// +build windows

package api

import (
	"reflect"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// LSAUnicodeString is used by various Local Security Authority (LSA) functions
// to specify a Unicode string.
type LSAUnicodeString struct {
	Length        uint16
	MaximumLength uint16
	Buffer        uintptr
}

// ReadLSAUnicodeString is used to read buffer and return a string. // #nosec
func ReadLSAUnicodeString(pHandle windows.Handle, lus *LSAUnicodeString) (string, error) {
	if lus.MaximumLength == 0 || lus.Length == 0 {
		return "", nil
	}
	// read data
	data := make([]byte, int(lus.MaximumLength))
	_, err := ReadProcessMemory(pHandle, lus.Buffer, &data[0], uintptr(lus.MaximumLength))
	if err != nil {
		return "", err
	}
	// make string
	var utf16Str []uint16
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&utf16Str))
	sh.Len = int(lus.Length / 2)
	sh.Cap = int(lus.Length / 2)
	sh.Data = uintptr(unsafe.Pointer(&data[:lus.Length][0]))
	return string(utf16.Decode(utf16Str)), nil
}
