package taskmgr

import (
	"context"
	"fmt"
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
