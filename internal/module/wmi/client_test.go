// +build windows

package wmi

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

const testPathWin32Process = "Win32_Process"

var testWQLWin32Process = BuildWQLStatement(testWin32ProcessStr{}, testPathWin32Process)

func testCreateClient(t *testing.T) *Client {
	client, err := NewClient("root\\cimv2", nil)
	require.NoError(t, err)
	return client
}

// for test wmi structure tag and simple test.
type testWin32ProcessStr struct {
	Name   string
	PID    uint32 `wmi:"ProcessId"`
	Ignore string `wmi:"-"`
}

func TestClient_Query(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("Win32_Process", func(t *testing.T) {
		client := testCreateClient(t)

		var processes []*testWin32ProcessStr

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

func TestClient_GetObject(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("Win32_Process", func(t *testing.T) {
		client := testCreateClient(t)

		object, err := client.GetObject(testPathWin32Process)
		require.NoError(t, err)
		defer object.Clear()

		fmt.Println("value:", object.Value())
		path, err := object.Path()
		require.NoError(t, err)
		fmt.Println("path:", path)

		client.Close()

		testsuite.IsDestroyed(t, client)
	})
}

type testWin32ProcessCreateInputStr struct {
	CommandLine      string
	CurrentDirectory string
	ProcessStartup   testWin32ProcessStartupStr `wmi:"ProcessStartupInformation"`

	Ignore string `wmi:"-"`
}

// must use Class field to create object, not use structure field like
// |class struct{} `wmi:"class_name"`| because for anko script.
type testWin32ProcessStartupStr struct {
	// class name
	Class string `wmi:"-"`

	// property
	X uint32
	Y uint32

	Ignore string `wmi:"-"`
}

type testWin32ProcessCreateOutputStr struct {
	PID uint32 `wmi:"ProcessId"`

	Ignore string `wmi:"-"`
}

type testWin32ProcessGetOwnerOutputStr struct {
	Domain string
	User   string

	Ignore string `wmi:"-"`
}

type testWin32ProcessTerminateInputStr struct {
	Reason uint32
}

func TestClient_ExecMethod(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testCreateClient(t)

	t.Run("Win32_Process", func(t *testing.T) {
		const (
			pathCreate = "Win32_Process"
			pathObject = "Win32_Process.Handle=\"%d\""

			methodCreate    = "Create"
			methodGetOwner  = "GetOwner"
			methodTerminate = "Terminate"
		)

		var (
			commandLine      = "cmd.exe"
			currentDirectory = "C:\\"
			className        = "Win32_ProcessStartup"
		)

		// create process
		createInput := testWin32ProcessCreateInputStr{
			CommandLine:      commandLine,
			CurrentDirectory: currentDirectory,
			ProcessStartup: testWin32ProcessStartupStr{
				Class: className,
				X:     200,
				Y:     200,
			},
		}
		var createOutput testWin32ProcessCreateOutputStr
		err := client.ExecMethod(pathCreate, methodCreate, createInput, &createOutput)
		require.NoError(t, err)
		fmt.Printf("PID: %d\n", createOutput.PID)
		require.Zero(t, createOutput.Ignore)

		path := fmt.Sprintf(pathObject, createOutput.PID)

		// get owner
		var getOwnerOutput testWin32ProcessGetOwnerOutputStr
		err = client.ExecMethod(path, methodGetOwner, nil, &getOwnerOutput)
		require.NoError(t, err)
		fmt.Printf("Domain: %s, User: %s\n", getOwnerOutput.Domain, getOwnerOutput.User)
		require.Zero(t, getOwnerOutput.Ignore)

		// terminate process
		terminateInput := testWin32ProcessTerminateInputStr{
			Reason: 1,
		}
		err = client.ExecMethod(path, methodTerminate, terminateInput, nil)
		require.NoError(t, err)
	})

	client.Close()

	testsuite.IsDestroyed(t, client)
}

func TestBuildWQLStatement(t *testing.T) {
	win32Process := struct {
		Name   string
		PID    uint32 `wmi:"ProcessId"`
		Ignore string `wmi:"-"`
	}{}
	wql := BuildWQLStatement(win32Process, testPathWin32Process)
	require.Equal(t, "select Name, ProcessId from Win32_Process", wql)
}