// +build windows

package wmi

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

var testWQLWin32Process = BuildWQLStatement(testWin32ProcessStr{}, "Win32_Process")

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

		testsuite.IsDestroyed(t, &processes)
	})
}

func TestClient_GetObject(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("Win32_Process", func(t *testing.T) {
		client := testCreateClient(t)

		object, err := client.GetObject("Win32_Process")
		require.NoError(t, err)

		fmt.Println("value:", object.Value())
		path, err := object.Path()
		require.NoError(t, err)
		fmt.Println("path:", path)

		client.Close()

		testsuite.IsDestroyed(t, client)

		object.Clear()

		testsuite.IsDestroyed(t, object)
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

		testsuite.IsDestroyed(t, &createOutput)

		// get owner
		var getOwnerOutput testWin32ProcessGetOwnerOutputStr
		err = client.ExecMethod(path, methodGetOwner, nil, &getOwnerOutput)
		require.NoError(t, err)
		fmt.Printf("Domain: %s, User: %s\n", getOwnerOutput.Domain, getOwnerOutput.User)
		require.Zero(t, getOwnerOutput.Ignore)
		testsuite.IsDestroyed(t, &getOwnerOutput)

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

func TestClient_setValue(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testCreateClient(t)

	t.Run("full type object", func(t *testing.T) {
		type full struct {
			// --------value--------
			Int8    int8
			Int16   int16
			Int32   int32
			Int64   int64
			Uint8   uint8
			Uint16  uint16
			Uint32  uint32
			Uint64  uint64
			Float32 float32
			Float64 float32
			Bool    bool
			String  string

			ByteSlice   []byte
			StringSlice []string

			DateTime  time.Time
			Reference string
			Char16    uint16

			Object struct {
				Class string `wmi:"-"`

				X uint32
				Y uint32
			}

			// --------pointer--------
			Int8Ptr    *int8
			Int16Ptr   *int16
			Int32Ptr   *int32
			Int64Ptr   *int64
			Uint8Ptr   *uint8
			Uint16Ptr  *uint16
			Uint32Ptr  *uint32
			Uint64Ptr  *uint64
			Float32Ptr *float32
			Float64Ptr *float32
			BoolPtr    *bool
			StringPtr  *string

			ByteSlicePtr   *[]byte
			StringSlicePtr *[]string

			DateTimePtr  *time.Time
			ReferencePtr string
			Char16Ptr    uint16

			ObjectPtr *struct {
				Class string `wmi:"-"`

				X uint32
				Y uint32
			}
		}

		object := new(Object)
		var pg *monkey.PatchGuard
		patch := func(obj *Object, name string) (*Object, error) {
			pg.Unpatch()
			defer pg.Restore()

			prop, err := obj.GetMethodInputParameters(name)
			require.NoError(t, err)

			// add fake properties that conatins all supported type
			err = obj.AddProperty("Int8", CIMTypeInt8, false)
			require.NoError(t, err)
			err = obj.AddProperty("Int16", CIMTypeInt16, false)
			require.NoError(t, err)
			err = obj.AddProperty("Int32", CIMTypeInt32, false)
			require.NoError(t, err)
			err = obj.AddProperty("Int64", CIMTypeInt64, false)
			require.NoError(t, err)
			err = obj.AddProperty("Uint8", CIMTypeUint8, false)
			require.NoError(t, err)
			err = obj.AddProperty("Uint16", CIMTypeUint16, false)
			require.NoError(t, err)
			err = obj.AddProperty("Uint32", CIMTypeUint32, false)
			require.NoError(t, err)
			err = obj.AddProperty("Uint64", CIMTypeUint64, false)
			require.NoError(t, err)
			err = obj.AddProperty("Float32", CIMTypeFloat32, false)
			require.NoError(t, err)
			err = obj.AddProperty("Float64", CIMTypeFloat64, false)
			require.NoError(t, err)
			err = obj.AddProperty("Bool", CIMTypeBool, false)
			require.NoError(t, err)
			err = obj.AddProperty("String", CIMTypeString, false)
			require.NoError(t, err)
			err = obj.AddProperty("ByteSlice", CIMTypeUint8, true)
			require.NoError(t, err)
			err = obj.AddProperty("StringSlice", CIMTypeString, true)
			require.NoError(t, err)
			err = obj.AddProperty("DateTime", CIMTypeDateTime, false)
			require.NoError(t, err)
			err = obj.AddProperty("Reference", CIMTypeReference, false)
			require.NoError(t, err)
			err = obj.AddProperty("Char16", CIMTypeChar16, false)
			require.NoError(t, err)
			err = obj.AddProperty("Object", CIMTypeObject, false)
			require.NoError(t, err)

			return prop, nil
		}
		pg = monkey.PatchInstanceMethod(object, "GetMethodInputParameters", patch)
		defer pg.Unpatch()

	})

	client.Close()

	testsuite.IsDestroyed(t, client)
}

func TestClient_Query_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("part", func(t *testing.T) {
		client := testCreateClient(t)

		query := func() {
			var processes []*testWin32ProcessStr

			err := client.Query(testWQLWin32Process, &processes)
			require.NoError(t, err)

			require.NotEmpty(t, processes)
			for _, process := range processes {
				require.NotZero(t, process.Name)
				require.Zero(t, process.Ignore)
			}

			testsuite.IsDestroyed(t, &processes)
		}
		testsuite.RunParallel(10, nil, nil, query, query)

		client.Close()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("whole", func(t *testing.T) {
		var client *Client

		init := func() {
			client = testCreateClient(t)
		}
		query := func() {
			var processes []*testWin32ProcessStr

			err := client.Query(testWQLWin32Process, &processes)
			require.NoError(t, err)

			require.NotEmpty(t, processes)
			for _, process := range processes {
				require.NotZero(t, process.Name)
				require.Zero(t, process.Ignore)
			}

			testsuite.IsDestroyed(t, &processes)
		}
		cleanup := func() {
			client.Close()
		}
		testsuite.RunParallel(10, init, cleanup, query, query)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_GetObject_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("part", func(t *testing.T) {
		client := testCreateClient(t)

		get := func() {
			object, err := client.GetObject("Win32_Process")
			require.NoError(t, err)

			require.NotZero(t, object.raw.Val)
			path, err := object.Path()
			require.NoError(t, err)
			require.NotZero(t, path)

			object.Clear()

			testsuite.IsDestroyed(t, object)
		}
		testsuite.RunParallel(10, nil, nil, get, get)

		client.Close()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("whole", func(t *testing.T) {
		var client *Client

		init := func() {
			client = testCreateClient(t)
		}
		query := func() {
			object, err := client.GetObject("Win32_Process")
			require.NoError(t, err)

			require.NotZero(t, object.raw.Val)
			path, err := object.Path()
			require.NoError(t, err)
			require.NotZero(t, path)

			object.Clear()

			testsuite.IsDestroyed(t, object)
		}
		cleanup := func() {
			client.Close()
		}
		testsuite.RunParallel(10, init, cleanup, query, query)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_ExecMethod_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

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

	t.Run("part", func(t *testing.T) {
		client := testCreateClient(t)

		exec := func() {
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

			testsuite.IsDestroyed(t, &createOutput)

			// get owner
			var getOwnerOutput testWin32ProcessGetOwnerOutputStr
			err = client.ExecMethod(path, methodGetOwner, nil, &getOwnerOutput)
			require.NoError(t, err)
			fmt.Printf("Domain: %s, User: %s\n", getOwnerOutput.Domain, getOwnerOutput.User)
			require.Zero(t, getOwnerOutput.Ignore)

			testsuite.IsDestroyed(t, &getOwnerOutput)

			// terminate process
			terminateInput := testWin32ProcessTerminateInputStr{
				Reason: 1,
			}
			err = client.ExecMethod(path, methodTerminate, terminateInput, nil)
			require.NoError(t, err)
		}
		testsuite.RunParallel(10, nil, nil, exec, exec)

		client.Close()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("whole", func(t *testing.T) {
		var client *Client

		init := func() {
			client = testCreateClient(t)
		}
		exec := func() {
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

			testsuite.IsDestroyed(t, &createOutput)

			// get owner
			var getOwnerOutput testWin32ProcessGetOwnerOutputStr
			err = client.ExecMethod(path, methodGetOwner, nil, &getOwnerOutput)
			require.NoError(t, err)
			fmt.Printf("Domain: %s, User: %s\n", getOwnerOutput.Domain, getOwnerOutput.User)
			require.Zero(t, getOwnerOutput.Ignore)

			testsuite.IsDestroyed(t, &getOwnerOutput)

			// terminate process
			terminateInput := testWin32ProcessTerminateInputStr{
				Reason: 1,
			}
			err = client.ExecMethod(path, methodTerminate, terminateInput, nil)
			require.NoError(t, err)
		}
		cleanup := func() {
			client.Close()
		}
		testsuite.RunParallel(10, init, cleanup, exec, exec)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

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

	t.Run("part", func(t *testing.T) {
		client := testCreateClient(t)

		query := func() {
			var processes []*testWin32ProcessStr

			err := client.Query(testWQLWin32Process, &processes)
			require.NoError(t, err)

			require.NotEmpty(t, processes)
			for _, process := range processes {
				require.NotZero(t, process.Name)
				require.Zero(t, process.Ignore)
			}

			testsuite.IsDestroyed(t, &processes)
		}
		get := func() {
			object, err := client.GetObject("Win32_Process")
			require.NoError(t, err)

			require.NotZero(t, object.raw.Val)
			path, err := object.Path()
			require.NoError(t, err)
			require.NotZero(t, path)

			object.Clear()

			testsuite.IsDestroyed(t, object)
		}
		exec := func() {
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

			testsuite.IsDestroyed(t, &createOutput)

			// get owner
			var getOwnerOutput testWin32ProcessGetOwnerOutputStr
			err = client.ExecMethod(path, methodGetOwner, nil, &getOwnerOutput)
			require.NoError(t, err)
			fmt.Printf("Domain: %s, User: %s\n", getOwnerOutput.Domain, getOwnerOutput.User)
			require.Zero(t, getOwnerOutput.Ignore)

			testsuite.IsDestroyed(t, &getOwnerOutput)

			// terminate process
			terminateInput := testWin32ProcessTerminateInputStr{
				Reason: 1,
			}
			err = client.ExecMethod(path, methodTerminate, terminateInput, nil)
			require.NoError(t, err)
		}
		testsuite.RunParallel(10, nil, nil, query, query, get, get, exec, exec)

		client.Close()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("whole", func(t *testing.T) {
		var client *Client

		init := func() {
			client = testCreateClient(t)
		}
		query := func() {
			var processes []*testWin32ProcessStr

			err := client.Query(testWQLWin32Process, &processes)
			require.NoError(t, err)

			require.NotEmpty(t, processes)
			for _, process := range processes {
				require.NotZero(t, process.Name)
				require.Zero(t, process.Ignore)
			}

			testsuite.IsDestroyed(t, &processes)
		}
		get := func() {
			object, err := client.GetObject("Win32_Process")
			require.NoError(t, err)

			require.NotZero(t, object.raw.Val)
			path, err := object.Path()
			require.NoError(t, err)
			require.NotZero(t, path)

			object.Clear()

			testsuite.IsDestroyed(t, object)
		}
		exec := func() {
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

			testsuite.IsDestroyed(t, &createOutput)

			// get owner
			var getOwnerOutput testWin32ProcessGetOwnerOutputStr
			err = client.ExecMethod(path, methodGetOwner, nil, &getOwnerOutput)
			require.NoError(t, err)
			fmt.Printf("Domain: %s, User: %s\n", getOwnerOutput.Domain, getOwnerOutput.User)
			require.Zero(t, getOwnerOutput.Ignore)

			testsuite.IsDestroyed(t, &getOwnerOutput)

			// terminate process
			terminateInput := testWin32ProcessTerminateInputStr{
				Reason: 1,
			}
			err = client.ExecMethod(path, methodTerminate, terminateInput, nil)
			require.NoError(t, err)
		}
		cleanup := func() {
			client.Close()
		}
		testsuite.RunParallel(10, init, cleanup, query, query, get, get, exec, exec)

		testsuite.IsDestroyed(t, client)
	})
}

func TestThread_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	testsuite.RunMultiTimes(10, func() {
		client := testCreateClient(t)

		testsuite.RunMultiTimes(10, func() {
			for i := 0; i < 10; i++ {
				var systemInfo []testWin32OperatingSystem

				err := client.Query("select * from Win32_OperatingSystem", &systemInfo)
				require.NoError(t, err)

				require.NotEmpty(t, systemInfo)
				for _, systemInfo := range systemInfo {
					testCheckOutputStructure(t, systemInfo)
				}

				testsuite.IsDestroyed(t, &systemInfo)
			}
		})

		client.Close()

		testsuite.IsDestroyed(t, client)
	})
}

func TestBuildWQLStatement(t *testing.T) {
	win32Process := struct {
		Name   string
		PID    uint32 `wmi:"ProcessId"`
		Ignore string `wmi:"-"`
	}{}
	wql := BuildWQLStatement(win32Process, "Win32_Process")
	require.Equal(t, "select Name, ProcessId from Win32_Process", wql)
}
