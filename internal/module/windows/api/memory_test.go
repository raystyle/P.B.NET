package api

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestReadProcessMemory(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handle := windows.CurrentProcess()
		var pbi ProcessBasicInformation
		ic := InfoClassProcessBasicInformation

		_, err := NTQueryInformationProcess(handle, ic, (*byte)(unsafe.Pointer(&pbi)), unsafe.Sizeof(pbi))
		require.NoError(t, err)

		buf := make([]byte, 16)
		n, err := ReadProcessMemory(handle, pbi.PEBBaseAddress, &buf[0], uintptr(len(buf)))
		require.NoError(t, err)
		require.Equal(t, len(buf), n)

		t.Log(buf)
	})
}
