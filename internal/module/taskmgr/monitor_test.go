package taskmgr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	handler := func(_ context.Context, event uint8, data interface{}) {
		switch event {
		case EventProcessCreated:
			testMonitorPrintCreatedProcesses(data.([]*Process))
		case EventProcessTerminated:
			testMonitorPrintTerminatedProcesses(data.([]*Process))
		}
	}
	monitor, err := NewMonitor(logger.Test, handler)
	require.NoError(t, err)

	monitor.SetInterval(500 * time.Millisecond)

	time.Sleep(5 * time.Second)

	monitor.GetProcesses()

	monitor.Close()

	testsuite.IsDestroyed(t, monitor)
}

func testMonitorPrintCreatedProcesses(processes []*Process) {
	for _, process := range processes {
		fmt.Printf("create process PID: %d Name: %s\n", process.PID, process.Name)
	}
}

func testMonitorPrintTerminatedProcesses(processes []*Process) {
	for _, process := range processes {
		fmt.Printf("terminate process PID: %d Name: %s\n", process.PID, process.Name)
	}
}

func TestMonitor_EventProcessCreated(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	path, err := os.Executable()
	require.NoError(t, err)
	name := filepath.Base(path)

	var created bool

	handler := func(_ context.Context, event uint8, data interface{}) {
		if event != EventProcessCreated {
			return
		}
		for _, process := range data.([]*Process) {
			if process.Name == name {
				created = true
			}
		}
	}
	monitor, err := NewMonitor(logger.Test, handler)
	require.NoError(t, err)

	// wait first auto refresh
	time.Sleep(2 * defaultRefreshInterval)

	// create process
	cmd := exec.Command(path)
	err = cmd.Start()
	require.NoError(t, err)

	// wait refresh
	time.Sleep(2 * defaultRefreshInterval)

	// terminate process
	err = cmd.Process.Kill()
	require.NoError(t, err)
	err = cmd.Process.Release()
	require.NoError(t, err)

	monitor.Close()

	testsuite.IsDestroyed(t, monitor)

	require.True(t, created, "not find expected created process")
}

func TestMonitor_EventProcessTerminated(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	path, err := os.Executable()
	require.NoError(t, err)
	name := filepath.Base(path)

	// create process
	cmd := exec.Command(path)
	err = cmd.Start()
	require.NoError(t, err)

	var terminated bool

	handler := func(_ context.Context, event uint8, data interface{}) {
		if event != EventProcessTerminated {
			return
		}
		for _, process := range data.([]*Process) {
			if process.Name == name {
				terminated = true
			}
		}
	}
	monitor, err := NewMonitor(logger.Test, handler)
	require.NoError(t, err)

	// wait first auto refresh
	time.Sleep(2 * defaultRefreshInterval)

	// terminate process
	err = cmd.Process.Kill()
	require.NoError(t, err)
	err = cmd.Process.Release()
	require.NoError(t, err)

	// wait refresh
	time.Sleep(2 * defaultRefreshInterval)

	monitor.Close()

	testsuite.IsDestroyed(t, monitor)

	require.True(t, terminated, "not find expected terminated process")
}
