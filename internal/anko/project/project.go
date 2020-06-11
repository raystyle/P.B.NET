// Package project generate by resource/code/anko/package.go, don't edit it.
package project

import (
	"reflect"

	"project/internal/patch/json"

	"github.com/mattn/anko/env"
)

func init() {
	initPatchJSON()
}

func initPatchJSON() {
	env.Packages["internal/patch/json"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"NewEncoder": reflect.ValueOf(json.NewEncoder),
		"NewDecoder": reflect.ValueOf(json.NewDecoder),
		"Marshal":    reflect.ValueOf(json.Marshal),
		"Unmarshal":  reflect.ValueOf(json.Unmarshal),
	}
	var (
		encoder json.Encoder
		decoder json.Decoder
	)
	env.PackageTypes["internal/patch/json"] = map[string]reflect.Type{
		"Encoder": reflect.TypeOf(&encoder).Elem(),
		"Decoder": reflect.TypeOf(&decoder).Elem(),
	}
}
