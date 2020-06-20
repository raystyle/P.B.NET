// Package thirdparty generate by script/code/anko/package.go, don't edit it.
package thirdparty

import (
	"reflect"

	"github.com/kardianos/service"
	"github.com/mattn/anko/env"
	"github.com/pelletier/go-toml"
	"github.com/vmihailenco/msgpack/v4"
)

func init() {
	initGithubComKardianosService()
	initGithubComPelletierGoTOML()
	initGithubComVmihailencoMsgpackV4()
}

func initGithubComKardianosService() {
	env.Packages["github.com/kardianos/service"] = map[string]reflect.Value{
		// define constants
		"StatusRunning": reflect.ValueOf(service.StatusRunning),
		"StatusStopped": reflect.ValueOf(service.StatusStopped),
		"StatusUnknown": reflect.ValueOf(service.StatusUnknown),

		// define variables
		"ConsoleLogger":              reflect.ValueOf(service.ConsoleLogger),
		"ControlAction":              reflect.ValueOf(service.ControlAction),
		"ErrNameFieldRequired":       reflect.ValueOf(service.ErrNameFieldRequired),
		"ErrNoServiceSystemDetected": reflect.ValueOf(service.ErrNoServiceSystemDetected),
		"ErrNotInstalled":            reflect.ValueOf(service.ErrNotInstalled),

		// define functions
		"AvailableSystems": reflect.ValueOf(service.AvailableSystems),
		"ChooseSystem":     reflect.ValueOf(service.ChooseSystem),
		"ChosenSystem":     reflect.ValueOf(service.ChosenSystem),
		"Control":          reflect.ValueOf(service.Control),
		"Interactive":      reflect.ValueOf(service.Interactive),
		"New":              reflect.ValueOf(service.New),
		"Platform":         reflect.ValueOf(service.Platform),
	}
	var (
		config        service.Config
		iface         service.Interface
		keyValue      service.KeyValue
		logger        service.Logger
		svc           service.Service
		status        service.Status
		system        service.System
		windowsLogger service.WindowsLogger
	)
	env.PackageTypes["github.com/kardianos/service"] = map[string]reflect.Type{
		"Config":        reflect.TypeOf(&config).Elem(),
		"Interface":     reflect.TypeOf(&iface).Elem(),
		"KeyValue":      reflect.TypeOf(&keyValue).Elem(),
		"Logger":        reflect.TypeOf(&logger).Elem(),
		"Service":       reflect.TypeOf(&svc).Elem(),
		"Status":        reflect.TypeOf(&status).Elem(),
		"System":        reflect.TypeOf(&system).Elem(),
		"WindowsLogger": reflect.TypeOf(&windowsLogger).Elem(),
	}
}

func initGithubComPelletierGoTOML() {
	env.Packages["github.com/pelletier/go-toml"] = map[string]reflect.Value{
		// define constants
		"OrderAlphabetical": reflect.ValueOf(toml.OrderAlphabetical),
		"OrderPreserve":     reflect.ValueOf(toml.OrderPreserve),

		// define variables

		// define functions
		"Load":               reflect.ValueOf(toml.Load),
		"LoadBytes":          reflect.ValueOf(toml.LoadBytes),
		"LoadFile":           reflect.ValueOf(toml.LoadFile),
		"LoadReader":         reflect.ValueOf(toml.LoadReader),
		"LocalDateOf":        reflect.ValueOf(toml.LocalDateOf),
		"LocalDateTimeOf":    reflect.ValueOf(toml.LocalDateTimeOf),
		"LocalTimeOf":        reflect.ValueOf(toml.LocalTimeOf),
		"Marshal":            reflect.ValueOf(toml.Marshal),
		"NewDecoder":         reflect.ValueOf(toml.NewDecoder),
		"NewEncoder":         reflect.ValueOf(toml.NewEncoder),
		"ParseLocalDate":     reflect.ValueOf(toml.ParseLocalDate),
		"ParseLocalDateTime": reflect.ValueOf(toml.ParseLocalDateTime),
		"ParseLocalTime":     reflect.ValueOf(toml.ParseLocalTime),
		"TreeFromMap":        reflect.ValueOf(toml.TreeFromMap),
		"Unmarshal":          reflect.ValueOf(toml.Unmarshal),
	}
	var (
		decoder       toml.Decoder
		encoder       toml.Encoder
		localDate     toml.LocalDate
		localDateTime toml.LocalDateTime
		localTime     toml.LocalTime
		marshaler     toml.Marshaler
		position      toml.Position
		setOptions    toml.SetOptions
		tree          toml.Tree
		unmarshaler   toml.Unmarshaler
	)
	env.PackageTypes["github.com/pelletier/go-toml"] = map[string]reflect.Type{
		"Decoder":       reflect.TypeOf(&decoder).Elem(),
		"Encoder":       reflect.TypeOf(&encoder).Elem(),
		"LocalDate":     reflect.TypeOf(&localDate).Elem(),
		"LocalDateTime": reflect.TypeOf(&localDateTime).Elem(),
		"LocalTime":     reflect.TypeOf(&localTime).Elem(),
		"Marshaler":     reflect.TypeOf(&marshaler).Elem(),
		"Position":      reflect.TypeOf(&position).Elem(),
		"SetOptions":    reflect.TypeOf(&setOptions).Elem(),
		"Tree":          reflect.TypeOf(&tree).Elem(),
		"Unmarshaler":   reflect.TypeOf(&unmarshaler).Elem(),
	}
}

func initGithubComVmihailencoMsgpackV4() {
	env.Packages["github.com/vmihailenco/msgpack/v4"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"Marshal":     reflect.ValueOf(msgpack.Marshal),
		"NewDecoder":  reflect.ValueOf(msgpack.NewDecoder),
		"NewEncoder":  reflect.ValueOf(msgpack.NewEncoder),
		"Register":    reflect.ValueOf(msgpack.Register),
		"RegisterExt": reflect.ValueOf(msgpack.RegisterExt),
		"Unmarshal":   reflect.ValueOf(msgpack.Unmarshal),
	}
	var (
		customDecoder msgpack.CustomDecoder
		customEncoder msgpack.CustomEncoder
		decoder       msgpack.Decoder
		encoder       msgpack.Encoder
		marshaler     msgpack.Marshaler
		unmarshaler   msgpack.Unmarshaler
	)
	env.PackageTypes["github.com/vmihailenco/msgpack/v4"] = map[string]reflect.Type{
		"CustomDecoder": reflect.TypeOf(&customDecoder).Elem(),
		"CustomEncoder": reflect.TypeOf(&customEncoder).Elem(),
		"Decoder":       reflect.TypeOf(&decoder).Elem(),
		"Encoder":       reflect.TypeOf(&encoder).Elem(),
		"Marshaler":     reflect.TypeOf(&marshaler).Elem(),
		"Unmarshaler":   reflect.TypeOf(&unmarshaler).Elem(),
	}
}
