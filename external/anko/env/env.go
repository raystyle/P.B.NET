package env

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// ExternalLookup for Env external lookup of values and types.
type ExternalLookup interface {
	Get(string) (reflect.Value, error)
	Type(string) (reflect.Type, error)
}

// Env is the environment needed for a VM to run in.
type Env struct {
	parent         *Env
	externalLookup ExternalLookup

	values  map[string]reflect.Value
	types   map[string]reflect.Type
	rwMutex *sync.RWMutex
}

var (
	// Packages is a where packages can be stored so VM import command can be
	// used to import them. reflect.Value must be valid or VM may crash.
	// For nil must use NilValue.
	Packages = make(map[string]map[string]reflect.Value)

	// PackageTypes is a where package types can be stored so VM import command
	// can be used to import them. reflect.Type must be valid or VM may crash.
	// For nil type must use NilType.
	PackageTypes = make(map[string]map[string]reflect.Type)

	// NilType is the reflect.type of nil.
	NilType = reflect.TypeOf(nil)

	// NilValue is the reflect.value of nil.
	NilValue = reflect.New(reflect.TypeOf((*interface{})(nil)).Elem()).Elem()

	// basic type in vm.
	basicTypes = map[string]reflect.Type{
		"interface": reflect.ValueOf([]interface{}{int64(1)}).Index(0).Type(),
		"bool":      reflect.TypeOf(true),
		"string":    reflect.TypeOf("a"),
		"int":       reflect.TypeOf(int(1)),
		"int32":     reflect.TypeOf(int32(1)),
		"int64":     reflect.TypeOf(int64(1)),
		"uint":      reflect.TypeOf(uint(1)),
		"uint32":    reflect.TypeOf(uint32(1)),
		"uint64":    reflect.TypeOf(uint64(1)),
		"byte":      reflect.TypeOf(byte(1)),
		"rune":      reflect.TypeOf('a'),
		"float32":   reflect.TypeOf(float32(1)),
		"float64":   reflect.TypeOf(float64(1)),
	}

	// ErrSymbolContainsDot symbol contains .
	ErrSymbolContainsDot = errors.New("symbol contains \".\"")
)

// NewEnv creates new global scope.
func NewEnv() *Env {
	return &Env{
		rwMutex: new(sync.RWMutex),
		values:  make(map[string]reflect.Value),
	}
}

// SetExternalLookup sets an external lookup.
func (e *Env) SetExternalLookup(externalLookup ExternalLookup) {
	e.externalLookup = externalLookup
}

// NewEnv creates new child scope.
func (e *Env) NewEnv() *Env {
	return &Env{
		rwMutex: new(sync.RWMutex),
		parent:  e,
		values:  make(map[string]reflect.Value),
	}
}

// String returns string of values and types in current scope.
func (e *Env) String() string {
	var buffer bytes.Buffer
	e.rwMutex.RLock()
	defer e.rwMutex.RUnlock()
	if e.parent == nil {
		buffer.WriteString("No parent\n")
	} else {
		buffer.WriteString("Has parent\n")
	}
	for symbol, value := range e.values {
		buffer.WriteString(fmt.Sprintf("%v = %#v\n", symbol, value))
	}
	for symbol, aType := range e.types {
		buffer.WriteString(fmt.Sprintf("%v = %v\n", symbol, aType))
	}
	return buffer.String()
}
