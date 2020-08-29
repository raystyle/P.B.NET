// +build windows

package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

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
			fmt.Println(pid)
		}

		testsuite.IsDestroyed(t, &pid)
	})
}
