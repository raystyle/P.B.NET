// +build windows

package goroot

import (
	"reflect"
	"syscall"

	"github.com/mattn/anko/env"
)

func init() {
	initSyscallWindows()
}

func initSyscallWindows() {
	env.Packages["syscall"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"LoadDLL":    reflect.ValueOf(syscall.LoadDLL),
		"NewLazyDLL": reflect.ValueOf(syscall.NewLazyDLL),
	}
	var (
		dll      syscall.DLL
		proc     syscall.Proc
		lazyDLL  syscall.LazyDLL
		lazyProc syscall.LazyProc
	)
	env.PackageTypes["syscall"] = map[string]reflect.Type{
		"DLL":      reflect.TypeOf(&dll).Elem(),
		"Proc":     reflect.TypeOf(&proc).Elem(),
		"LazyDLL":  reflect.TypeOf(&lazyDLL).Elem(),
		"LazyProc": reflect.TypeOf(&lazyProc).Elem(),
	}
}
