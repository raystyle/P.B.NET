// +build windows

package wmi

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

const (
	testPathWin32Process = "Win32_Process"
	testWQLWin32Process  = "select Name, ProcessId from Win32_Process"
)

// for test wmi structure tag and simple test.
type testWin32Process struct {
	Name   string
	PID    uint32 `wmi:"ProcessId"`
	Ignore string `wmi:"-"`
}

func testCreateClient(t *testing.T) *Client {
	client, err := NewClient("root\\cimv2", nil)
	require.NoError(t, err)
	return client
}

func TestClient_Query(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("Win32_Process", func(t *testing.T) {
		client := testCreateClient(t)

		var processes []*testWin32Process

		err := client.Query(testWQLWin32Process, &processes)
		require.NoError(t, err)

		client.Close()

		testsuite.IsDestroyed(t, client)

		require.NotEmpty(t, processes)
		for _, process := range processes {
			fmt.Printf("name: %s pid: %d\n", process.Name, process.PID)
			require.Zero(t, process.Ignore)
		}
	})
}

func TestClient_Get(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("Win32_Process", func(t *testing.T) {
		client := testCreateClient(t)

		object, err := client.GetObject(testPathWin32Process)
		require.NoError(t, err)

		fmt.Println(object.Value())
		object.Clear()

		client.Close()

		testsuite.IsDestroyed(t, client)
	})

}

func TestClient_ExecMethod(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("path without dot", func(t *testing.T) {
		client := testCreateClient(t)

		err := client.ExecMethod(testPathWin32Process, "Create", nil, "cmd.exe")
		require.NoError(t, err)

		client.Close()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("path with dot", func(t *testing.T) {
		client := testCreateClient(t)

		err := client.ExecMethod("win32_process.Handle=\"388\"", "GetOwner", nil, nil)
		require.NoError(t, err)

		client.Close()

		testsuite.IsDestroyed(t, client)
	})
}

func TestBuildWQL(t *testing.T) {
	wql := BuildWQL(testWin32Process{}, "Win32_Process")
	require.Equal(t, "select Name, ProcessId from Win32_Process", wql)
}
