// +build windows

package api

import (
	"fmt"
	"os"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"

	"project/internal/testsuite"
)

func TestIsWow64Process(t *testing.T) {
	isWow64, err := IsWow64Process(windows.CurrentProcess())
	require.NoError(t, err)
	fmt.Println("is wow64:", isWow64)
}

func TestGetProcessList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		processes, err := GetProcessList()
		require.NoError(t, err)

		fmt.Println("Name    PID    PPID")
		for _, process := range processes {
			fmt.Printf("%s %d %d\n", process.Name, process.PID, process.PPID)
		}

		testsuite.IsDestroyed(t, &processes)
	})
}

func TestGetProcessIDByName(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pid, err := GetProcessIDByName("lsass.exe")
		require.NoError(t, err)

		require.NotEmpty(t, pid)
		for _, pid := range pid {
			t.Log("pid:", pid)
		}

		testsuite.IsDestroyed(t, &pid)
	})
}

func TestOpenProcess(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handle, err := OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, uint32(os.Getpid()))
		require.NoError(t, err)

		CloseHandle(handle)
	})
}

func TestNTQueryInformationProcess(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handle := windows.CurrentProcess()
		var pbi ProcessBasicInformation
		ic := InfoClassProcessBasicInformation

		_, err := NTQueryInformationProcess(handle, ic, (*byte)(unsafe.Pointer(&pbi)), unsafe.Sizeof(pbi))
		require.NoError(t, err)

		t.Logf("0x%X\n", pbi.PEBBaseAddress)
	})
}

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
