// +build windows

package taskmgr

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/module/windows/privilege"
	"project/internal/testsuite"
)

func TestTaskList_GetProcesses(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	err := privilege.EnableDebugPrivilege()
	require.NoError(t, err)

	tasklist, err := newTaskList()
	require.NoError(t, err)

	processes, err := tasklist.GetProcesses()
	require.NoError(t, err)

	require.NotEmpty(t, processes)
	for _, process := range processes {
		fmt.Println(process.Name, process.Architecture, process.Username)
	}

	tasklist.Close()

	testsuite.IsDestroyed(t, tasklist)
}
