// +build windows

package wmi

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func testCheckStructure(t *testing.T, value interface{}) {
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
		require.Equal(t, reflect.Ptr, fieldPtr.Kind(), fieldPtrName)
		require.Equal(t, field.Interface(), fieldPtr.Elem().Interface(), fieldName)
	}
}

type testStruct struct {
	A     int16
	ABPtr *int16

	// slice
	S    []string
	SPtr *[]string

	// struct
}

func TestCheckStructure(t *testing.T) {
	a := int16(16)
	s := []string{"S"}

	as := testStruct{
		A:     16,
		ABPtr: &a,
		S:     []string{"S"},
		SPtr:  &s,
	}

	testCheckStructure(t, as)
	testCheckStructure(t, &as)
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
			testCheckStructure(t, systemInfo)
		}
	})

	t.Run("pointer", func(t *testing.T) {
		var systemInfo []*testWin32OperatingSystem

		err := client.Query(wql, &systemInfo)
		require.NoError(t, err)

		require.NotEmpty(t, systemInfo)
		for _, systemInfo := range systemInfo {
			testCheckStructure(t, systemInfo)
		}
	})

	client.Close()

	testsuite.IsDestroyed(t, client)
}

type testWin32ProcessCreateInput struct {
	CommandLine      string
	CurrentDirectory string
	ProcessStartup   testWin32ProcessStartup `wmi:"ProcessStartupInformation"`
}

// must use Class field to create object, not use structure field like
// |class struct{} `wmi:"class_name"`| because for anko script.
type testWin32ProcessStartup struct {
	Class string `wmi:"-"`
	X     uint32
	Y     uint32
}

type testWin32ProcessCreateInputPtr struct {
	CommandLine      *string
	CurrentDirectory *string
	ProcessStartup   *testWin32ProcessStartupPtr `wmi:"ProcessStartupInformation"`
}

type testWin32ProcessStartupPtr struct {
	Class string `wmi:"-"`
	X     *uint32
	Y     *uint32
}

type testWin32ProcessCreateOutput struct {
	PID    uint32  `wmi:"ProcessId"`
	PIDPtr *uint32 `wmi:"ProcessId"`

	ReturnValue    uint32
	ReturnValuePtr *uint32 `wmi:"ReturnValue"`
}

type testWin32ProcessGetOwnerOutput struct {
	Domain    string
	DomainPtr *string `wmi:"Domain"`

	User    string
	UserPtr *string `wmi:"User"`
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
				X:     50,
				Y:     50,
			},
		}
		var createOutput testWin32ProcessCreateOutput
		err := client.ExecMethod(pathCreate, methodCreate, createInput, &createOutput)
		require.NoError(t, err)

		fmt.Printf("PID: %d\n", createOutput.PID)
		testCheckStructure(t, createOutput)

		path := fmt.Sprintf(pathObject, createOutput.PID)

		// get owner
		var getOwnerOutput testWin32ProcessGetOwnerOutput
		err = client.ExecMethod(path, methodGetOwner, nil, &getOwnerOutput)
		require.NoError(t, err)
		fmt.Printf("Domain: %s, User: %s\n", getOwnerOutput.Domain, getOwnerOutput.User)
		testCheckStructure(t, getOwnerOutput)

		// terminate process
		terminateInput := testWin32ProcessTerminateInput{
			Reason: 1,
		}
		err = client.ExecMethod(path, methodTerminate, terminateInput, nil)
		require.NoError(t, err)
	})

	t.Run("pointer", func(t *testing.T) {
		// create process
		x := uint32(50)
		y := uint32(50)
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
		testCheckStructure(t, &createOutput)

		path := fmt.Sprintf(pathObject, createOutput.PID)

		// get owner
		var getOwnerOutput testWin32ProcessGetOwnerOutput
		err = client.ExecMethod(path, methodGetOwner, nil, &getOwnerOutput)
		require.NoError(t, err)
		fmt.Printf("Domain: %s, User: %s\n", getOwnerOutput.Domain, getOwnerOutput.User)
		testCheckStructure(t, getOwnerOutput)

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
