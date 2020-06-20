package goroot

import (
	"reflect"
	"syscall"

	"github.com/mattn/anko/env"
)

func init() {
	initSyscall()
}

func initSyscall() {
	env.Packages["syscall"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"ByteSliceFromString": reflect.ValueOf(syscall.ByteSliceFromString),
		"BytePtrFromString":   reflect.ValueOf(syscall.BytePtrFromString),
		"Syscall":             reflect.ValueOf(syscall.Syscall),
	}
	var ()
	env.PackageTypes["syscall"] = map[string]reflect.Type{}
}
