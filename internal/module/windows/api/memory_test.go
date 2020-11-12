package api

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestReadProcessMemory(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handle := windows.CurrentProcess()

		info, err := NTQueryInformationProcess(handle, InfoClassProcessBasicInformation)
		require.NoError(t, err)
		pbi := info.(*ProcessBasicInformation)

		buf := make([]byte, 16)
		n, err := ReadProcessMemory(handle, pbi.PEBBaseAddress, &buf[0], uintptr(len(buf)))
		require.NoError(t, err)
		require.Equal(t, len(buf), n)

		t.Log(buf)
	})
}
