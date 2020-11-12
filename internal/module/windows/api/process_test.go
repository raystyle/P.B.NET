// +build windows

package api

import (
	"fmt"
	"os"
	"testing"

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
		hProcess, err := OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, uint32(os.Getpid()))
		require.NoError(t, err)

		CloseHandle(hProcess)
	})
}

func TestNTQueryInformationProcess(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		hProcess := windows.CurrentProcess()

		info, err := NTQueryInformationProcess(hProcess, InfoClassProcessBasicInformation)
		require.NoError(t, err)
		pbi := info.(*ProcessBasicInformation)

		t.Logf("0x%X\n", pbi.PEBBaseAddress)
	})
}
