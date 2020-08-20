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
		// check field value
		switch field.Type().Kind() {
		case reflect.Slice:
			require.NotZero(t, field.Len())
			require.NotZero(t, field.Index(0).Interface())
		default:
			require.NotZero(t, field.Interface())
		}
		// deep equal
		require.Equal(t, reflect.Ptr, fieldPtr.Kind())
		require.Equal(t, field.Interface(), fieldPtr.Elem().Interface())
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
}

type testWin32ProcessCreateOutput struct {
	PID         uint32 `wmi:"ProcessId"`
	ReturnValue uint32
}

type testWin32ProcessGetOwnerOutput struct {
	Domain string
	User   string
}

type testWin32ProcessTerminateInput struct {
	Reason uint32
}

func TestParseExecMethodResult(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testCreateClient(t)

	const (
		createPath = "Win32_Process"
		objectPath = "Win32_Process.Handle=\"%d\""
	)

	// first create a process, then terminate it

	t.Run("value", func(t *testing.T) {
		// create process
		createInput := testWin32ProcessCreateInput{
			CommandLine:      "cmd.exe",
			CurrentDirectory: "C:\\",
		}
		var createOutput testWin32ProcessCreateOutput
		err := client.ExecMethod(createPath, "Create", createInput, &createOutput)
		require.NoError(t, err)
		fmt.Println(createOutput)

		path := fmt.Sprintf(objectPath, createOutput.PID)

		// get owner
		var getOwnerOutput testWin32ProcessGetOwnerOutput
		err = client.ExecMethod(path, "GetOwner", nil, &getOwnerOutput)
		require.NoError(t, err)
		fmt.Println(getOwnerOutput)

		// terminate process
		terminateInput := testWin32ProcessTerminateInput{
			Reason: 1,
		}
		err = client.ExecMethod(path, "Terminate", terminateInput, nil)
		require.NoError(t, err)
	})

	t.Run("pointer", func(t *testing.T) {
		// create process
		createInput := testWin32ProcessCreateInput{
			CommandLine:      "cmd.exe",
			CurrentDirectory: "C:\\",
		}
		var createOutput testWin32ProcessCreateOutput
		err := client.ExecMethod(createPath, "Create", &createInput, &createOutput)
		require.NoError(t, err)
		fmt.Println(createOutput)

		path := fmt.Sprintf(objectPath, createOutput.PID)

		// get owner
		var getOwnerOutput testWin32ProcessGetOwnerOutput
		err = client.ExecMethod(path, "GetOwner", nil, &getOwnerOutput)
		require.NoError(t, err)
		fmt.Println(getOwnerOutput)

		// terminate process
		terminateInput := testWin32ProcessTerminateInput{
			Reason: 1,
		}
		err = client.ExecMethod(path, "Terminate", &terminateInput, nil)
		require.NoError(t, err)
	})

	client.Close()

	testsuite.IsDestroyed(t, client)
}
