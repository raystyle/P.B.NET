// +build windows

package wmi

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

const (
	testWQLWin32Process = "select Name, ProcessId from Win32_Process"
)

type testWin32Process struct {
	Name   string
	PID    uint32 `wmi:"ProcessId"`
	Ignore string `wmi:"-"`
}

func testCreateClient(t *testing.T) *Client {
	client, err := NewClient("", "root\\cimv2")
	require.NoError(t, err)
	return client
}

func TestClient_Query(t *testing.T) {
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
