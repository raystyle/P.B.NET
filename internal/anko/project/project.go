// Package project generate by resource/code/anko/package.go, don't edit it.
package project

import (
	"reflect"

	"project/internal/patch/json"
	"project/internal/patch/msgpack"
	"project/internal/patch/toml"

	"github.com/mattn/anko/env"
)

func init() {
	initInternalPatchJSON()
	initInternalPatchMsgpack()
	initInternalPatchToml()
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

func initInternalPatchMsgpack() {
	env.Packages["project/internal/patch/msgpack"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"Marshal":    reflect.ValueOf(msgpack.Marshal),
		"NewDecoder": reflect.ValueOf(msgpack.NewDecoder),
		"NewEncoder": reflect.ValueOf(msgpack.NewEncoder),
		"Unmarshal":  reflect.ValueOf(msgpack.Unmarshal),
	}
	var (
		decoder msgpack.Decoder
		encoder msgpack.Encoder
	)
	env.PackageTypes["project/internal/patch/msgpack"] = map[string]reflect.Type{
		"Decoder": reflect.TypeOf(&decoder).Elem(),
		"Encoder": reflect.TypeOf(&encoder).Elem(),
	}
}

func initInternalPatchToml() {
	env.Packages["project/internal/patch/toml"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"Marshal":   reflect.ValueOf(toml.Marshal),
		"Unmarshal": reflect.ValueOf(toml.Unmarshal),
	}
	var ()
	env.PackageTypes["project/internal/patch/toml"] = map[string]reflect.Type{}
}
