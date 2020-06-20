// Package project generate by resource/code/anko/package.go, don't edit it.
package project

import (
	"reflect"

	"project/internal/convert"
	"project/internal/httptool"
	"project/internal/logger"
	"project/internal/nettool"
	"project/internal/patch/json"
	"project/internal/patch/msgpack"
	"project/internal/patch/toml"
	"project/internal/xpanic"
	"project/internal/xreflect"

	"github.com/mattn/anko/env"
)

func init() {
	initInternalPatchJSON()
	initInternalPatchMsgpack()
	initInternalPatchToml()
	initInternalConvert()
	initInternalHTTPTool()
	initInternalLogger()
	initInternalNetTool()
	initInternalXPanic()
	initInternalXReflect()
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

func initInternalConvert() {
	env.Packages["project/internal/convert"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"AbsInt64":          reflect.ValueOf(convert.AbsInt64),
		"ByteSliceToString": reflect.ValueOf(convert.ByteSliceToString),
		"ByteToString":      reflect.ValueOf(convert.ByteToString),
		"BytesToFloat32":    reflect.ValueOf(convert.BytesToFloat32),
		"BytesToFloat64":    reflect.ValueOf(convert.BytesToFloat64),
		"BytesToInt16":      reflect.ValueOf(convert.BytesToInt16),
		"BytesToInt32":      reflect.ValueOf(convert.BytesToInt32),
		"BytesToInt64":      reflect.ValueOf(convert.BytesToInt64),
		"BytesToUint16":     reflect.ValueOf(convert.BytesToUint16),
		"BytesToUint32":     reflect.ValueOf(convert.BytesToUint32),
		"BytesToUint64":     reflect.ValueOf(convert.BytesToUint64),
		"Float32ToBytes":    reflect.ValueOf(convert.Float32ToBytes),
		"Float64ToBytes":    reflect.ValueOf(convert.Float64ToBytes),
		"FormatNumber":      reflect.ValueOf(convert.FormatNumber),
		"Int16ToBytes":      reflect.ValueOf(convert.Int16ToBytes),
		"Int32ToBytes":      reflect.ValueOf(convert.Int32ToBytes),
		"Int64ToBytes":      reflect.ValueOf(convert.Int64ToBytes),
		"Uint16ToBytes":     reflect.ValueOf(convert.Uint16ToBytes),
		"Uint32ToBytes":     reflect.ValueOf(convert.Uint32ToBytes),
		"Uint64ToBytes":     reflect.ValueOf(convert.Uint64ToBytes),
	}
	var ()
	env.PackageTypes["project/internal/convert"] = map[string]reflect.Type{}
}

func initInternalHTTPTool() {
	env.Packages["project/internal/httptool"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"FprintRequest":        reflect.ValueOf(httptool.FprintRequest),
		"NewSubHTTPFileSystem": reflect.ValueOf(httptool.NewSubHTTPFileSystem),
		"PrintRequest":         reflect.ValueOf(httptool.PrintRequest),
	}
	var ()
	env.PackageTypes["project/internal/httptool"] = map[string]reflect.Type{}
}

func initInternalLogger() {
	env.Packages["project/internal/logger"] = map[string]reflect.Value{
		// define constants
		"Debug":      reflect.ValueOf(logger.Debug),
		"Error":      reflect.ValueOf(logger.Error),
		"Exploit":    reflect.ValueOf(logger.Exploit),
		"Fatal":      reflect.ValueOf(logger.Fatal),
		"Info":       reflect.ValueOf(logger.Info),
		"Off":        reflect.ValueOf(logger.Off),
		"TimeLayout": reflect.ValueOf(logger.TimeLayout),
		"Warning":    reflect.ValueOf(logger.Warning),

		// define variables
		"Common":  reflect.ValueOf(logger.Common),
		"Discard": reflect.ValueOf(logger.Discard),
		"Test":    reflect.ValueOf(logger.Test),

		// define functions
		"Conn":                reflect.ValueOf(logger.Conn),
		"HijackLogWriter":     reflect.ValueOf(logger.HijackLogWriter),
		"NewMultiLogger":      reflect.ValueOf(logger.NewMultiLogger),
		"NewWriterWithPrefix": reflect.ValueOf(logger.NewWriterWithPrefix),
		"Parse":               reflect.ValueOf(logger.Parse),
		"Prefix":              reflect.ValueOf(logger.Prefix),
		"Wrap":                reflect.ValueOf(logger.Wrap),
	}
	var (
		level       logger.Level
		levelSetter logger.LevelSetter
		lg          logger.Logger
		multiLogger logger.MultiLogger
	)
	env.PackageTypes["project/internal/logger"] = map[string]reflect.Type{
		"Level":       reflect.TypeOf(&level).Elem(),
		"LevelSetter": reflect.TypeOf(&levelSetter).Elem(),
		"Logger":      reflect.TypeOf(&lg).Elem(),
		"MultiLogger": reflect.TypeOf(&multiLogger).Elem(),
	}
}

func initInternalNetTool() {
	env.Packages["project/internal/nettool"] = map[string]reflect.Value{
		// define constants

		// define variables
		"ErrEmptyPort": reflect.ValueOf(nettool.ErrEmptyPort),

		// define functions
		"CheckPort":             reflect.ValueOf(nettool.CheckPort),
		"CheckPortString":       reflect.ValueOf(nettool.CheckPortString),
		"DeadlineConn":          reflect.ValueOf(nettool.DeadlineConn),
		"DecodeExternalAddress": reflect.ValueOf(nettool.DecodeExternalAddress),
		"EncodeExternalAddress": reflect.ValueOf(nettool.EncodeExternalAddress),
		"IPEnabled":             reflect.ValueOf(nettool.IPEnabled),
		"IPToHost":              reflect.ValueOf(nettool.IPToHost),
		"IsNetClosingError":     reflect.ValueOf(nettool.IsNetClosingError),
		"SplitHostPort":         reflect.ValueOf(nettool.SplitHostPort),
	}
	var ()
	env.PackageTypes["project/internal/nettool"] = map[string]reflect.Type{}
}

func initInternalXPanic() {
	env.Packages["project/internal/xpanic"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"Error":      reflect.ValueOf(xpanic.Error),
		"Log":        reflect.ValueOf(xpanic.Log),
		"Print":      reflect.ValueOf(xpanic.Print),
		"PrintPanic": reflect.ValueOf(xpanic.PrintPanic),
		"PrintStack": reflect.ValueOf(xpanic.PrintStack),
	}
	var ()
	env.PackageTypes["project/internal/xpanic"] = map[string]reflect.Type{}
}

func initInternalXReflect() {
	env.Packages["project/internal/xreflect"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"GetStructureName":          reflect.ValueOf(xreflect.GetStructureName),
		"StructureToMap":            reflect.ValueOf(xreflect.StructureToMap),
		"StructureToMapWithoutZero": reflect.ValueOf(xreflect.StructureToMapWithoutZero),
	}
	var ()
	env.PackageTypes["project/internal/xreflect"] = map[string]reflect.Type{}
}
