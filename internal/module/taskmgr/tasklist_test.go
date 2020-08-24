package taskmgr

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestTaskList_GetProcesses(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	tasklist, err := newTaskList()
	require.NoError(t, err)

	processes, err := tasklist.GetProcesses()
	require.NoError(t, err)

	require.NotEmpty(t, processes)
	for i := 0; i < len(processes); i++ {
		spew.Dump(processes[i])
	}

	tasklist.Close()

	testsuite.IsDestroyed(t, tasklist)
}
