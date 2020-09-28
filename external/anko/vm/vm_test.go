package vm

import (
	"reflect"
)

type (
	testStruct1 struct {
		aInterface interface{}
		aBool      bool
		aInt32     int32
		aInt64     int64
		aFloat32   float32
		aFloat64   float32
		aString    string
		aFunc      func()

		aPtrInterface      *interface{}
		aPtrBool           *bool
		aPtrInt32          *int32
		aPtrInt64          *int64
		aPtrFloat32        *float32
		aPtrFloat64        *float32
		aPtrString         *string
		aPtrSliceInterface *[]interface{}
		aPtrSliceBool      *[]bool
		aPtrSliceInt32     *[]int32
		aPtrSliceInt64     *[]int64
		aPtrSliceFloat32   *[]float32
		aPtrSliceFloat64   *[]float32
		aPtrSliceString    *[]string

		aSliceInterface    []interface{}
		aSliceBool         []bool
		aSliceInt32        []int32
		aSliceInt64        []int64
		aSliceFloat32      []float32
		aSliceFloat64      []float32
		aSliceString       []string
		aSlicePtrInterface []*interface{}
		aSlicePtrBool      []*bool
		aSlicePtrInt32     []*int32
		aSlicePtrInt64     []*int64
		aSlicePtrFloat32   []*float32
		aSlicePtrFloat64   []*float32
		aSlicePtrString    []*string

		aMapInterface    map[string]interface{}
		aMapBool         map[string]bool
		aMapInt32        map[string]int32
		aMapInt64        map[string]int64
		aMapFloat32      map[string]float32
		aMapFloat64      map[string]float32
		aMapString       map[string]string
		aMapPtrInterface map[string]*interface{}
		aMapPtrBool      map[string]*bool
		aMapPtrInt32     map[string]*int32
		aMapPtrInt64     map[string]*int64
		aMapPtrFloat32   map[string]*float32
		aMapPtrFloat64   map[string]*float32
		aMapPtrString    map[string]*string

		aChanInterface    chan interface{}
		aChanBool         chan bool
		aChanInt32        chan int32
		aChanInt64        chan int64
		aChanFloat32      chan float32
		aChanFloat64      chan float32
		aChanString       chan string
		aChanPtrInterface chan *interface{}
		aChanPtrBool      chan *bool
		aChanPtrInt32     chan *int32
		aChanPtrInt64     chan *int64
		aChanPtrFloat32   chan *float32
		aChanPtrFloat64   chan *float32
		aChanPtrString    chan *string

		aPtrStruct *testStruct1
	}
	testStruct2 struct {
		aStruct testStruct1
	}
)

var (
	testVarValue    = reflect.Value{}
	testVarBool     = true
	testVarBoolP    = &testVarBool
	testVarInt32    = int32(1)
	testVarInt32P   = &testVarInt32
	testVarInt64    = int64(1)
	testVarInt64P   = &testVarInt64
	testVarFloat32  = float32(1)
	testVarFloat32P = &testVarFloat32
	testVarFloat64  = float64(1)
	testVarFloat64P = &testVarFloat64
	testVarString   = "a"
	testVarStringP  = &testVarString
	testVarFunc     = func() int64 { return 1 }
	testVarFuncP    = &testVarFunc

	testVarValueBool    = reflect.ValueOf(true)
	testVarValueInt32   = reflect.ValueOf(int32(1))
	testVarValueInt64   = reflect.ValueOf(int64(1))
	testVarValueFloat32 = reflect.ValueOf(float32(1.1))
	testVarValueFloat64 = reflect.ValueOf(1.1)
	testVarValueString  = reflect.ValueOf("a")

	testSliceEmpty []interface{}
	testSlice      = []interface{}{nil, true, int64(1), 1.1, "a"}
	testMapEmpty   map[interface{}]interface{}
	testMap        = map[interface{}]interface{}{"a": nil, "b": true, "c": int64(1), "d": 1.1, "e": "e"}
)
