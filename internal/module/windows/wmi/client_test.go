// +build windows

package wmi

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
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

	t.Run("common", func(t *testing.T) {
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

	t.Run("fail", func(t *testing.T) {
		client := testCreateClient(t)

		err := client.Query("invalid wql", nil)
		require.Error(t, err)

		client.Close()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("query after client closed", func(t *testing.T) {
		client := testCreateClient(t)

		client.Close()
		// make sure query queue is full
		for i := 0; i < 16; i++ {
			errCh := make(chan error, 1)
			client.queryQueue <- &execQuery{
				WQL: "invalid wql",
				Err: errCh,
			}
		}

		err := client.Query("invalid wql", nil)
		require.Error(t, err)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("failed to get result", func(t *testing.T) {
		client := testCreateClient(t)

		client.Close()
		// query will block because client will not handle it
		client.stopSignal = make(chan struct{})
		go func() {
			time.Sleep(time.Second)
			close(client.stopSignal)
		}()

		err := client.Query("invalid wql", nil)
		require.Error(t, err)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_GetObject(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
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

	t.Run("fail", func(t *testing.T) {
		client := testCreateClient(t)

		_, err := client.GetObject("invalid path")
		require.Error(t, err)

		client.Close()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("get after client closed", func(t *testing.T) {
		client := testCreateClient(t)

		client.Close()
		// make sure get queue is full
		for i := 0; i < 16; i++ {
			result := make(chan *getObjectResult, 1)
			client.getQueue <- &getObject{
				Path:   "invalid path",
				Result: result,
			}
		}

		_, err := client.GetObject("invalid path")
		require.Error(t, err)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("failed to get result", func(t *testing.T) {
		client := testCreateClient(t)

		client.Close()
		// query will block because client will not handle it
		client.stopSignal = make(chan struct{})
		go func() {
			time.Sleep(time.Second)
			close(client.stopSignal)
		}()

		_, err := client.GetObject("invalid path")
		require.Error(t, err)

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

	t.Run("common", func(t *testing.T) {
		client := testCreateClient(t)

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

		client.Close()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("fail", func(t *testing.T) {
		client := testCreateClient(t)

		err := client.ExecMethod("invalid path", "", nil, nil)
		require.Error(t, err)

		client.Close()

		testsuite.IsDestroyed(t, client)
	})

	t.Run("exec after client closed", func(t *testing.T) {
		client := testCreateClient(t)

		client.Close()
		// make sure get queue is full
		for i := 0; i < 16; i++ {
			errCh := make(chan error, 1)
			client.execQueue <- &execMethod{
				Path: "invalid path",
				Err:  errCh,
			}
		}

		err := client.ExecMethod("invalid path", "", nil, nil)
		require.Error(t, err)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("failed to get result", func(t *testing.T) {
		client := testCreateClient(t)

		client.Close()
		// query will block because client will not handle it
		client.stopSignal = make(chan struct{})
		go func() {
			time.Sleep(time.Second)
			close(client.stopSignal)
		}()

		err := client.ExecMethod("invalid path", "", nil, nil)
		require.Error(t, err)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_init(t *testing.T) {
	client := Client{}
	client.opts = new(Options)

	t.Run("call CoInitializeEx in same thread", func(t *testing.T) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		err := client.init()
		require.NoError(t, err)
		err = client.init()
		require.NoError(t, err)

		ole.CoUninitialize()
	})

	testsuite.IsDestroyed(t, &client)
}

func TestClient_setExecMethodInput(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		path   = "Win32_Process"
		method = "Create"
	)

	client := testCreateClient(t)

	t.Run("nil pointer", func(t *testing.T) {
		var input *int
		err := client.ExecMethod(path, method, input, nil)
		require.Error(t, err)
	})

	t.Run("pointer not point to structure", func(t *testing.T) {
		input := 0
		err := client.ExecMethod(path, method, &input, nil)
		require.Error(t, err)
	})

	t.Run("invalid input type", func(t *testing.T) {
		err := client.ExecMethod(path, method, "foo", nil)
		require.Error(t, err)
	})

	client.Close()

	testsuite.IsDestroyed(t, client)
}

type testFullType struct {
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
	Float64 float64
	Bool    bool
	String  string

	// ByteSlice   []byte
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
	Float64Ptr *float64
	BoolPtr    *bool
	StringPtr  *string

	// ByteSlicePtr   *[]byte
	StringSlicePtr *[]string

	DateTimePtr  *time.Time
	ReferencePtr *string
	Char16Ptr    *uint16

	ObjectPtr *struct {
		Class string `wmi:"-"`

		X *uint32
		Y *uint32
	}
}

func testGenerateFullType() *testFullType {
	Int8 := int8(123)
	Int16 := int16(-12345)
	Int32 := int32(-1234567)
	Int64 := int64(-12345678901111)
	Uint8 := uint8(123)
	Uint16 := uint16(12345)
	Uint32 := uint32(123456)
	Uint64 := uint64(12345678901111)
	Float32 := float32(123.1234)
	Float64 := 123.123456789
	var Bool bool // IDE bug
	String := "full"

	// byteSlice := []byte{1, 2, 3, 4}
	stringSlice := []string{"1", "2", "3", "4"}

	DateTime := time.Now()
	Reference := "path"
	Char16 := uint16(1234)
	Object := struct {
		Class string `wmi:"-"`
		X     uint32
		Y     uint32
	}{Class: "Win32_ProcessStartup"}
	ObjectPtr := &struct {
		Class string `wmi:"-"`
		X     *uint32
		Y     *uint32
	}{Class: "Win32_ProcessStartup"}

	return &testFullType{
		// --------value--------
		Int8:    Int8,
		Int16:   Int16,
		Int32:   Int32,
		Int64:   Int64,
		Uint8:   Uint8,
		Uint16:  Uint16,
		Uint32:  Uint32,
		Uint64:  Uint64,
		Float32: Float32,
		Float64: Float64,
		Bool:    Bool,
		String:  String,

		// ByteSlice:   byteSlice,
		StringSlice: stringSlice,

		DateTime:  DateTime,
		Reference: Reference,
		Char16:    Char16,
		Object:    Object,

		// --------pointer--------
		Int8Ptr:    &Int8,
		Int16Ptr:   &Int16,
		Int32Ptr:   &Int32,
		Int64Ptr:   &Int64,
		Uint8Ptr:   &Uint8,
		Uint16Ptr:  &Uint16,
		Uint32Ptr:  &Uint32,
		Uint64Ptr:  &Uint64,
		Float32Ptr: &Float32,
		Float64Ptr: &Float64,
		BoolPtr:    &Bool,
		StringPtr:  &String,

		// ByteSlicePtr:   &byteSlice,
		StringSlicePtr: &stringSlice,

		DateTimePtr:  &DateTime,
		ReferencePtr: &Reference,
		Char16Ptr:    &Char16,
		ObjectPtr:    ObjectPtr,
	}
}

// add fake properties that contains all supported type.
func testAddFullTypeProperties(t *testing.T, client *Client, obj *Object) {
	// add values
	Int8 := int8(123)
	Int16 := int16(-12345)
	Int32 := int32(-1234567)
	Int64 := int64(-12345678901111)
	Uint8 := uint8(123)
	Uint16 := uint16(12345)
	Uint32 := uint32(123456)
	Uint64 := uint64(12345678901111)
	Float32 := float32(123.1234)
	Float64 := 123.123456789
	var Bool bool // IDE bug
	String := "full"

	// byteSlice := []byte{1, 2, 3, 4}
	stringSlice := []string{"1", "2", "3", "4"}

	DateTime := time.Now()
	Reference := "path"
	Char16 := uint16(1234)
	// don't use client.GetObject() or will block
	class, err := oleutil.CallMethod(client.wmi, "Get", "Win32_ProcessStartup")
	require.NoError(t, err)
	Object := &Object{raw: class}

	for _, item := range [...]*struct {
		Name    string
		Type    uint8
		IsArray bool
		Value   interface{}
	}{
		// --------value--------
		{"Int8", CIMTypeInt8, false, Int8},
		{"Int16", CIMTypeInt16, false, Int16},
		{"Int32", CIMTypeInt32, false, Int32},
		{"Int64", CIMTypeInt64, false, Int64},
		{"Uint8", CIMTypeUint8, false, Uint8},
		{"Uint16", CIMTypeUint16, false, Uint16},
		{"Uint32", CIMTypeUint32, false, Uint32},
		{"Uint64", CIMTypeUint64, false, Uint64},
		{"Float32", CIMTypeFloat32, false, Float32},
		{"Float64", CIMTypeFloat64, false, Float64},
		{"Bool", CIMTypeBool, false, Bool},
		{"String", CIMTypeString, false, String},
		{"ByteSlice", CIMTypeUint8, true, nil},
		{"StringSlice", CIMTypeString, true, stringSlice},
		{"DateTime", CIMTypeDateTime, false, DateTime},
		{"Reference", CIMTypeReference, false, Reference},
		{"Char16", CIMTypeChar16, false, Char16},
		{"Object", CIMTypeObject, false, Object},

		// --------pointer--------
		{"Int8Ptr", CIMTypeInt8, false, Int8},
		{"Int16Ptr", CIMTypeInt16, false, Int16},
		{"Int32Ptr", CIMTypeInt32, false, Int32},
		{"Int64Ptr", CIMTypeInt64, false, Int64},
		{"Uint8Ptr", CIMTypeUint8, false, Uint8},
		{"Uint16Ptr", CIMTypeUint16, false, Uint16},
		{"Uint32Ptr", CIMTypeUint32, false, Uint32},
		{"Uint64Ptr", CIMTypeUint64, false, Uint64},
		{"Float32Ptr", CIMTypeFloat32, false, Float32},
		{"Float64Ptr", CIMTypeFloat64, false, Float64},
		{"BoolPtr", CIMTypeBool, false, Bool},
		{"StringPtr", CIMTypeString, false, String},
		{"ByteSlicePtr", CIMTypeUint8, true, nil},
		{"StringSlicePtr", CIMTypeString, true, stringSlice},
		{"DateTimePtr", CIMTypeDateTime, false, DateTime},
		{"ReferencePtr", CIMTypeReference, false, Reference},
		{"Char16Ptr", CIMTypeChar16, false, Char16},
		{"ObjectPtr", CIMTypeObject, false, Object},
	} {
		err := obj.AddProperty(item.Name, item.Type, item.IsArray)
		require.NoError(t, err)
		err = obj.SetProperty(item.Name, item.Value)
		require.NoError(t, err)
	}
}

func TestClient_setValue(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		path   = "Win32_Process"
		method = "Create"
	)

	client := testCreateClient(t)

	t.Run("full type object", func(t *testing.T) {
		object := new(Object)
		var pg *monkey.PatchGuard
		patch := func(obj *Object, name string) (*Object, error) {
			pg.Unpatch()
			defer pg.Restore()

			prop, err := obj.GetMethodInputParameters(name)
			require.NoError(t, err)

			testAddFullTypeProperties(t, client, prop)
			return prop, nil
		}
		pg = monkey.PatchInstanceMethod(object, "GetMethodInputParameters", patch)
		defer pg.Unpatch()

		input := testGenerateFullType()

		err := client.ExecMethod(path, method, input, nil)
		require.NoError(t, err)
	})

	t.Run("invalid slice type", func(t *testing.T) {
		input := struct {
			ByteSlice []byte
		}{}

		err := client.ExecMethod(path, method, &input, nil)
		require.Error(t, err)
	})

	t.Run("unsupported field type", func(t *testing.T) {
		input := struct {
			Chan chan struct{}
		}{}

		err := client.ExecMethod(path, method, &input, nil)
		require.Error(t, err)
	})

	client.Close()

	testsuite.IsDestroyed(t, client)
}

func TestClient_setStruct(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const (
		path   = "Win32_Process"
		method = "Create"
	)

	client := testCreateClient(t)

	t.Run("no class field", func(t *testing.T) {
		input := struct {
			Object struct {
			}
		}{}

		err := client.ExecMethod(path, method, &input, nil)
		require.Error(t, err)
	})

	t.Run("class field not string", func(t *testing.T) {
		input := struct {
			Object struct {
				Class int `wmi:"-"`
			}
		}{}

		err := client.ExecMethod(path, method, &input, nil)
		require.Error(t, err)
	})

	t.Run("class field empty string", func(t *testing.T) {
		input := struct {
			Object struct {
				Class string `wmi:"-"`
			}
		}{}

		err := client.ExecMethod(path, method, &input, nil)
		require.Error(t, err)
	})

	t.Run("invalid class", func(t *testing.T) {
		input := struct {
			Object struct {
				Class string `wmi:"-"`
			}
		}{}
		input.Object.Class = "invalid class"

		err := client.ExecMethod(path, method, &input, nil)
		require.Error(t, err)
	})

	t.Run("failed to set object field", func(t *testing.T) {
		input := struct {
			Object struct {
				Class string `wmi:"-"`

				NotExist bool
			}
		}{}
		input.Object.Class = "Win32_ProcessStartup"

		err := client.ExecMethod(path, method, &input, nil)
		require.Error(t, err)
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

func TestMultiClient(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("success", func(t *testing.T) {
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
	})

	t.Run("fail", func(t *testing.T) {
		testsuite.RunMultiTimes(10, func() {
			client := testCreateClient(t)

			testsuite.RunMultiTimes(10, func() {
				for i := 0; i < 10; i++ {
					var systemInfo []testWin32OperatingSystem

					err := client.Query("select * from", &systemInfo)
					require.Error(t, err)

					testsuite.IsDestroyed(t, &systemInfo)
				}
			})

			client.Close()

			testsuite.IsDestroyed(t, client)
		})
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
