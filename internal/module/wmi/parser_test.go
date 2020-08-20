// +build windows

package wmi

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

type testStruct struct {
	A     int16
	ABPtr *int16
}

func TestCheckStructure(t *testing.T) {
	ab := new(int16)
	*ab = int16(16)
	as := testStruct{
		A:     16,
		ABPtr: ab,
	}
	testCheckStructure(t, as)
	testCheckStructure(t, &as)

	// ab := new(int16)
	//	*ab = int16(1)
	//	as := testStruct{
	//		A:     2,
	//		ABPtr: ab,
	//	}
	//	testCheckStructure(t, as)
	//	testCheckStructure(t, &as)

	// ab := new(int16)
	//	*ab = int16(1)
	//	as := testStruct{
	//		A:     0,
	//		ABPtr: ab,
	//	}
	//	testCheckStructure(t, as)
	//	testCheckStructure(t, &as)
}

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
	// check a multiple of two
	require.Equal(t, 0, l%2, "structure field is not a multiple of two")
	// compare value
	for i := 0; i < l; i += 2 {
		field := val.Field(i)
		fieldPtr := val.Field(i + 1)

		require.NotZero(t, field.Interface())

		require.Equal(t, reflect.Ptr, fieldPtr.Kind())
		require.Equal(t, field.Interface(), fieldPtr.Elem().Interface())
	}
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

	InstallDate    time.Duration
	InstallDatePtr *time.Duration `wmi:"InstallDate"`

	MUILanguage    []string
	MUILanguagePtr *[]string `wmi:"MUILanguage"`
}

func TestParseExecQueryResult(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("Win32_OperatingSystem", func(t *testing.T) {
		client := testCreateClient(t)

		systemInfo := testWin32OperatingSystem{}

		err := client.Query("select * from Win32_OperatingSystem", &systemInfo)
		require.NoError(t, err)

		client.Close()

		testsuite.IsDestroyed(t, client)

	})
}

func TestParseMethodQueryResult(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

}
