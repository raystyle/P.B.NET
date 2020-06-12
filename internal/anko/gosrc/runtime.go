// Package gosrc generate by resource/code/anko/package.go, don't edit it.
package gosrc

import (
	"reflect"
	"runtime"

	"github.com/mattn/anko/env"
)

func init() {
	initRuntime()
}

func initRuntime() {
	env.Packages["runtime"] = map[string]reflect.Value{
		// define constants
		"GOOS":   reflect.ValueOf(runtime.GOOS),
		"GOARCH": reflect.ValueOf(runtime.GOARCH),

		// define variables

		// define functions
		"GC":         reflect.ValueOf(runtime.GC),
		"GOMAXPROCS": reflect.ValueOf(runtime.GOMAXPROCS),
		"GOROOT":     reflect.ValueOf(runtime.GOROOT),
		"Version":    reflect.ValueOf(runtime.Version),
	}
	var ()
	env.PackageTypes["runtime"] = map[string]reflect.Type{}
}
