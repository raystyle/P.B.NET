// Package project generate by resource/code/anko/package.go, don't edit it.
package project

import (
	"reflect"

	"project/internal/patch/json"

	"github.com/mattn/anko/env"
)

func init() {
	initInternalPatchJSON()
}

func initInternalPatchJSON() {
	env.Packages["project/internal/patch/json"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"Marshal":    reflect.ValueOf(json.Marshal),
		"NewDecoder": reflect.ValueOf(json.NewDecoder),
		"NewEncoder": reflect.ValueOf(json.NewEncoder),
		"Unmarshal":  reflect.ValueOf(json.Unmarshal),
	}
	var (
		decoder json.Decoder
		encoder json.Encoder
	)
	env.PackageTypes["project/internal/patch/json"] = map[string]reflect.Type{
		"Decoder": reflect.TypeOf(&decoder).Elem(),
		"Encoder": reflect.TypeOf(&encoder).Elem(),
	}
}
