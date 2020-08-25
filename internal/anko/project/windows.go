// +build windows

// Package project generate by script/code/anko/package.go, don't edit it.
package project

import (
	"reflect"

	"github.com/mattn/anko/env"

	"project/internal/module/windows/privilege"
	"project/internal/module/windows/wmi"
)

func init() {
	initInternalModuleWindowsWMI()
	initInternalModuleWindowsPrivilege()
}

func initInternalModuleWindowsWMI() {
	env.Packages["project/internal/module/windows/wmi"] = map[string]reflect.Value{
		// define constants
		"CIMTypeBool":      reflect.ValueOf(wmi.CIMTypeBool),
		"CIMTypeChar16":    reflect.ValueOf(wmi.CIMTypeChar16),
		"CIMTypeDateTime":  reflect.ValueOf(wmi.CIMTypeDateTime),
		"CIMTypeFloat32":   reflect.ValueOf(wmi.CIMTypeFloat32),
		"CIMTypeFloat64":   reflect.ValueOf(wmi.CIMTypeFloat64),
		"CIMTypeInt16":     reflect.ValueOf(wmi.CIMTypeInt16),
		"CIMTypeInt32":     reflect.ValueOf(wmi.CIMTypeInt32),
		"CIMTypeInt64":     reflect.ValueOf(wmi.CIMTypeInt64),
		"CIMTypeInt8":      reflect.ValueOf(wmi.CIMTypeInt8),
		"CIMTypeObject":    reflect.ValueOf(wmi.CIMTypeObject),
		"CIMTypeReference": reflect.ValueOf(wmi.CIMTypeReference),
		"CIMTypeString":    reflect.ValueOf(wmi.CIMTypeString),
		"CIMTypeUint16":    reflect.ValueOf(wmi.CIMTypeUint16),
		"CIMTypeUint32":    reflect.ValueOf(wmi.CIMTypeUint32),
		"CIMTypeUint64":    reflect.ValueOf(wmi.CIMTypeUint64),
		"CIMTypeUint8":     reflect.ValueOf(wmi.CIMTypeUint8),

		// define variables

		// define functions
		"BuildWQLStatement": reflect.ValueOf(wmi.BuildWQLStatement),
		"NewClient":         reflect.ValueOf(wmi.NewClient),
	}
	var (
		client           wmi.Client
		errFieldMismatch wmi.ErrFieldMismatch
		object           wmi.Object
		options          wmi.Options
	)
	env.PackageTypes["project/internal/module/windows/wmi"] = map[string]reflect.Type{
		"Client":           reflect.TypeOf(&client).Elem(),
		"ErrFieldMismatch": reflect.TypeOf(&errFieldMismatch).Elem(),
		"Object":           reflect.TypeOf(&object).Elem(),
		"Options":          reflect.TypeOf(&options).Elem(),
	}
}

func initInternalModuleWindowsPrivilege() {
	env.Packages["project/internal/module/windows/privilege"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"EnableDebugPrivilege": reflect.ValueOf(privilege.EnableDebugPrivilege),
	}
	var ()
	env.PackageTypes["project/internal/module/windows/privilege"] = map[string]reflect.Type{}
}
