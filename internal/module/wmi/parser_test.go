// +build windows

package wmi

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestWMIDateTimeToTime(t *testing.T) {
	t.Run("common", func(t *testing.T) {
		now := time.Now()
		datetime := timeToWMIDateTime(now)
		ti, err := wmiDateTimeToTime(datetime)
		require.NoError(t, err)
		require.True(t, now.Sub(ti) < time.Second)
	})

	t.Run("invalid time string length", func(t *testing.T) {
		_, err := wmiDateTimeToTime("foo")
		require.Error(t, err)
	})

	t.Run("invalid time string", func(t *testing.T) {
		_, err := wmiDateTimeToTime(strings.Repeat("a", 25))
		require.Error(t, err)
	})
}

func testCheckOutputStructure(t *testing.T, value interface{}) {
	val := reflect.ValueOf(value)
	typ := reflect.TypeOf(value)
	if typ.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		t.Fatal("value is not a structure or pointer")
	}
	// check number of field
	l := typ.NumField()
	require.NotEqual(t, 0, l, "empty structure")
	// check structure field is a multiple of two
	require.Equal(t, 0, l%2, "fields not a multiple of two")
	// compare value
	for i := 0; i < l; i += 2 {
		field := val.Field(i)
		fieldPtr := val.Field(i + 1)
		fieldName := typ.Field(i).Name
		fieldPtrName := typ.Field(i + 1).Name
		// skip ignore field
		if typ.Field(i).Tag.Get("wmi") == "-" {
			continue
		}
		// check field value, skip "ReturnValue" field
		if fieldName != "ReturnValue" {
			switch field.Type().Kind() {
			case reflect.Slice:
				require.NotZero(t, field.Len(), fieldName)
				require.NotZero(t, field.Index(0).Interface(), fieldName)
			default:
				require.NotZero(t, field.Interface(), fieldName)
			}
		}
		// deep equal value
		require.Equal(t, reflect.Ptr, fieldPtr.Kind(), fieldPtrName, "is not pointer")
		require.Equal(t, field.Interface(), fieldPtr.Elem().Interface(), fieldName)
	}
}

type testStruct struct {
	A     int16
	ABPtr *int16

	// slice
	S    []string
	SPtr *[]string

	Str    testInnerStruct
	StrPtr *testInnerStruct
}

type testInnerStruct struct {
	A    string
	APtr *string
}

func TestCheckOutputStructure(t *testing.T) {
	a := int16(16)
	s := []string{"S"}

	as := testStruct{
		A:     16,
		ABPtr: &a,
		S:     []string{"S"},
		SPtr:  &s,
		Str: testInnerStruct{
			A:    "Str",
			APtr: &s[0],
		},
		StrPtr: &testInnerStruct{
			A:    "Str",
			APtr: &s[0],
		},
	}
	testCheckOutputStructure(t, as)
	testCheckOutputStructure(t, &as)
}

// for test structure field types, dont worried the same structure tag.
type testWin32OperatingSystem struct {
	CurrentTimeZone    int16
	CurrentTimeZonePtr *int16 `wmi:"CurrentTimeZone"`

	ForegroundApplicationBoost    uint8
	ForegroundApplicationBoostPtr *uint8 `wmi:"ForegroundApplicationBoost"`

	OSType    uint16
	OSTypePtr *uint16 `wmi:"OSType"`

	NumberOfProcesses    uint32
	NumberOfProcessesPtr *uint32 `wmi:"NumberOfProcesses"`

	FreeVirtualMemory    uint64
	FreeVirtualMemoryPtr *uint64 `wmi:"FreeVirtualMemory"`

	Primary    bool
	PrimaryPtr *bool `wmi:"Primary"`

	CSName    string
	CSNamePtr *string `wmi:"CSName"`

	InstallDate    time.Time
	InstallDatePtr *time.Time `wmi:"InstallDate"`

	MUILanguages    []string
	MUILanguagesPtr *[]string `wmi:"MUILanguages"`

	Ignore    string `wmi:"-"`
	IgnorePtr string `wmi:"-"`
}

func TestParseExecQueryResult(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testCreateClient(t)

	const wql = "select * from Win32_OperatingSystem"

	t.Run("value", func(t *testing.T) {
		var systemInfo []testWin32OperatingSystem

		err := client.Query(wql, &systemInfo)
		require.NoError(t, err)

		require.NotEmpty(t, systemInfo)
		for _, systemInfo := range systemInfo {
			testCheckOutputStructure(t, systemInfo)
		}

		testsuite.IsDestroyed(t, &systemInfo)
	})

	t.Run("pointer", func(t *testing.T) {
		var systemInfo []*testWin32OperatingSystem

		err := client.Query(wql, &systemInfo)
		require.NoError(t, err)

		require.NotEmpty(t, systemInfo)
		for _, systemInfo := range systemInfo {
			testCheckOutputStructure(t, systemInfo)
		}

		testsuite.IsDestroyed(t, &systemInfo)
	})

	client.Close()

	testsuite.IsDestroyed(t, client)
}

type testWin32ProcessCreateInput struct {
	CommandLine      string
	CurrentDirectory string
	ProcessStartup   testWin32ProcessStartup `wmi:"ProcessStartupInformation"`

	Ignore string `wmi:"-"`
}

type testWin32ProcessStartup struct {
	Class string `wmi:"-"`

	X uint32
	Y uint32

	Ignore string `wmi:"-"`
}

type testWin32ProcessCreateInputPtr struct {
	CommandLine      *string
	CurrentDirectory *string
	ProcessStartup   *testWin32ProcessStartupPtr `wmi:"ProcessStartupInformation"`

	Ignore *string `wmi:"-"`
}

type testWin32ProcessStartupPtr struct {
	Class string `wmi:"-"`
	X     *uint32
	Y     *uint32

	Ignore *string `wmi:"-"`
}

type testWin32ProcessCreateOutput struct {
	PID    uint32  `wmi:"ProcessId"`
	PIDPtr *uint32 `wmi:"ProcessId"`

	ReturnValue    uint32
	ReturnValuePtr *uint32 `wmi:"ReturnValue"`

	Ignore    string `wmi:"-"`
	IgnorePtr string `wmi:"-"`
}

type testWin32ProcessGetOwnerOutput struct {
	Domain    string
	DomainPtr *string `wmi:"Domain"`

	User    string
	UserPtr *string `wmi:"User"`

	Ignore    string `wmi:"-"`
	IgnorePtr string `wmi:"-"`
}

type testWin32ProcessTerminateInput struct {
	Reason uint32
}

func TestParseExecMethodResult(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

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

	t.Run("value", func(t *testing.T) {
		// create process
		createInput := testWin32ProcessCreateInput{
			CommandLine:      commandLine,
			CurrentDirectory: currentDirectory,
			ProcessStartup: testWin32ProcessStartup{
				Class: className,
				X:     200,
				Y:     200,
			},
		}
		var createOutput testWin32ProcessCreateOutput
		err := client.ExecMethod(pathCreate, methodCreate, createInput, &createOutput)
		require.NoError(t, err)
		fmt.Printf("PID: %d\n", createOutput.PID)
		testCheckOutputStructure(t, createOutput)

		path := fmt.Sprintf(pathObject, createOutput.PID)

		testsuite.IsDestroyed(t, &createOutput)

		// get owner
		var getOwnerOutput testWin32ProcessGetOwnerOutput
		err = client.ExecMethod(path, methodGetOwner, nil, &getOwnerOutput)
		require.NoError(t, err)
		fmt.Printf("Domain: %s, User: %s\n", getOwnerOutput.Domain, getOwnerOutput.User)
		testCheckOutputStructure(t, getOwnerOutput)
		testsuite.IsDestroyed(t, &getOwnerOutput)

		// terminate process
		terminateInput := testWin32ProcessTerminateInput{
			Reason: 1,
		}
		err = client.ExecMethod(path, methodTerminate, terminateInput, nil)
		require.NoError(t, err)
	})

	t.Run("pointer", func(t *testing.T) {
		// create process
		x := uint32(200)
		y := uint32(200)
		createInput := testWin32ProcessCreateInputPtr{
			CommandLine:      &commandLine,
			CurrentDirectory: &currentDirectory,
			ProcessStartup: &testWin32ProcessStartupPtr{
				Class: className,
				X:     &x,
				Y:     &y,
			},
		}
		var createOutput testWin32ProcessCreateOutput
		err := client.ExecMethod(pathCreate, methodCreate, &createInput, &createOutput)
		require.NoError(t, err)
		fmt.Printf("PID: %d\n", createOutput.PID)
		testCheckOutputStructure(t, &createOutput)

		path := fmt.Sprintf(pathObject, createOutput.PID)

		testsuite.IsDestroyed(t, &createOutput)

		// get owner
		var getOwnerOutput testWin32ProcessGetOwnerOutput
		err = client.ExecMethod(path, methodGetOwner, nil, &getOwnerOutput)
		require.NoError(t, err)
		fmt.Printf("Domain: %s, User: %s\n", getOwnerOutput.Domain, getOwnerOutput.User)
		testCheckOutputStructure(t, &getOwnerOutput)
		testsuite.IsDestroyed(t, &getOwnerOutput)

		// terminate process
		terminateInput := testWin32ProcessTerminateInput{
			Reason: 1,
		}
		err = client.ExecMethod(path, methodTerminate, &terminateInput, nil)
		require.NoError(t, err)
	})

	t.Run("nil pointer", func(t *testing.T) {
		// create process
		createInput := testWin32ProcessCreateInputPtr{
			CommandLine:      &commandLine,
			CurrentDirectory: &currentDirectory,
		}
		var createOutput testWin32ProcessCreateOutput
		err := client.ExecMethod(pathCreate, methodCreate, &createInput, &createOutput)
		require.NoError(t, err)
		fmt.Printf("PID: %d\n", createOutput.PID)
		testCheckOutputStructure(t, &createOutput)

		path := fmt.Sprintf(pathObject, createOutput.PID)

		testsuite.IsDestroyed(t, &createOutput)

		// get owner
		var getOwnerOutput testWin32ProcessGetOwnerOutput
		err = client.ExecMethod(path, methodGetOwner, nil, &getOwnerOutput)
		require.NoError(t, err)
		fmt.Printf("Domain: %s, User: %s\n", getOwnerOutput.Domain, getOwnerOutput.User)
		testCheckOutputStructure(t, &getOwnerOutput)
		testsuite.IsDestroyed(t, &getOwnerOutput)

		// terminate process
		terminateInput := testWin32ProcessTerminateInput{
			Reason: 1,
		}
		err = client.ExecMethod(path, methodTerminate, &terminateInput, nil)
		require.NoError(t, err)
	})

	client.Close()

	testsuite.IsDestroyed(t, client)
}

func TestWalkStruct(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testCreateClient(t)

	t.Run("full type", func(t *testing.T) {
		object, err := client.GetObject("Win32_Process")
		require.NoError(t, err)
		defer object.Clear()
		testAddFullTypeProperties(t, client, object)
		fullType := testGenerateFullType()

		err = parseExecMethodOutput(object, fullType)
		require.NoError(t, err)
	})

	t.Run("failed to get property", func(t *testing.T) {
		object, err := client.GetObject("Win32_Process")
		require.NoError(t, err)
		defer object.Clear()

		output := struct {
			Foo string
		}{}
		err = parseExecMethodOutput(object, &output)
		require.Error(t, err)
	})

	client.Close()

	testsuite.IsDestroyed(t, client)
}

func TestSetValue(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testCreateClient(t)

	object, err := client.GetObject("Win32_Process")
	require.NoError(t, err)
	defer object.Clear()
	testAddFullTypeProperties(t, client, object)

	t.Run("failed to set int value", func(t *testing.T) {
		output := struct {
			Int8 string
		}{}
		err = parseExecMethodOutput(object, &output)
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("failed to set uint value", func(t *testing.T) {
		output := struct {
			Uint8  uint8
			Uint81 int8 `wmi:"Uint8"`
			Uint16 string
		}{}
		err = parseExecMethodOutput(object, &output)
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("failed to set float value", func(t *testing.T) {
		output := struct {
			Float32 string
		}{}
		err = parseExecMethodOutput(object, &output)
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("failed to set bool value", func(t *testing.T) {
		output := struct {
			Bool string
		}{}
		err = parseExecMethodOutput(object, &output)
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("failed to set string value", func(t *testing.T) {
		t.Run("int", func(t *testing.T) {
			output := struct {
				String int
			}{}
			err = parseExecMethodOutput(object, &output)
			require.Error(t, err)
			t.Log(err)
		})

		t.Run("uint", func(t *testing.T) {
			output := struct {
				String uint
			}{}
			err = parseExecMethodOutput(object, &output)
			require.Error(t, err)
			t.Log(err)
		})

		t.Run("time", func(t *testing.T) {
			output := struct {
				String time.Time
			}{}
			err = parseExecMethodOutput(object, &output)
			require.Error(t, err)
			t.Log(err)
		})

		t.Run("time", func(t *testing.T) {
			output := struct {
				String struct{}
			}{}
			err = parseExecMethodOutput(object, &output)
			require.Error(t, err)
			t.Log(err)
		})

		t.Run("unsupported type", func(t *testing.T) {
			output := struct {
				String chan struct{}
			}{}
			err = parseExecMethodOutput(object, &output)
			require.Error(t, err)
			t.Log(err)
		})
	})

	client.Close()

	testsuite.IsDestroyed(t, client)
}
