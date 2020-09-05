package anko

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/mattn/anko/env"
)

type runtime struct {
	// store values
	values    map[string]reflect.Value
	valuesRWM sync.RWMutex

	// store types
	types    map[string]reflect.Type
	typesRWM sync.RWMutex
}

func (rt *runtime) Get(symbol string) (reflect.Value, error) {
	rt.valuesRWM.RLock()
	defer rt.valuesRWM.RUnlock()
	if value, ok := rt.values[symbol]; ok {
		return value, nil
	}
	return reflect.Value{}, fmt.Errorf("value %q is not defined", symbol)
}

func (rt *runtime) Type(symbol string) (reflect.Type, error) {
	rt.typesRWM.RLock()
	defer rt.typesRWM.RUnlock()
	if typ, ok := rt.types[symbol]; ok {
		return typ, nil
	}
	return nil, fmt.Errorf("type %q is not defined", symbol)
}

func (rt *runtime) DefineValue(symbol string, value interface{}) error {
	var reflectValue reflect.Value
	if value == nil {
		reflectValue = env.NilValue
	} else {
		var ok bool
		reflectValue, ok = value.(reflect.Value)
		if !ok {
			reflectValue = reflect.ValueOf(value)
		}
	}
	return rt.defineValue(symbol, reflectValue)
}

func (rt *runtime) defineValue(symbol string, value reflect.Value) error {
	if strings.Contains(symbol, ".") {
		return env.ErrSymbolContainsDot
	}
	rt.valuesRWM.Lock()
	defer rt.valuesRWM.Unlock()
	rt.values[symbol] = value
	return nil
}

func (rt *runtime) DefineType(symbol string, typ interface{}) error {
	var reflectType reflect.Type
	if typ == nil {
		reflectType = env.NilType
	} else {
		var ok bool
		reflectType, ok = typ.(reflect.Type)
		if !ok {
			reflectType = reflect.TypeOf(typ)
		}
	}
	return rt.defineType(symbol, reflectType)
}

func (rt *runtime) defineType(symbol string, typ reflect.Type) error {
	if strings.Contains(symbol, ".") {
		return env.ErrSymbolContainsDot
	}
	rt.typesRWM.Lock()
	defer rt.typesRWM.Unlock()
	rt.types[symbol] = typ
	return nil
}

// Env is the environment needed for a VM to run in.
type Env struct {
	*env.Env
	runtime *runtime
}

func (e *Env) Close() {

}
