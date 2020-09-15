package env

import (
	"fmt"
	"reflect"
	"strings"
)

// DefineType defines type in current scope.
func (e *Env) DefineType(symbol string, typ interface{}) error {
	var reflectType reflect.Type
	if typ == nil {
		reflectType = NilType
	} else {
		var ok bool
		reflectType, ok = typ.(reflect.Type)
		if !ok {
			reflectType = reflect.TypeOf(typ)
		}
	}
	return e.DefineReflectType(symbol, reflectType)
}

// DefineReflectType defines type in current scope.
func (e *Env) DefineReflectType(symbol string, typ reflect.Type) error {
	if strings.Contains(symbol, ".") {
		return ErrSymbolContainsDot
	}
	e.rwm.Lock()
	defer e.rwm.Unlock()
	if e.types == nil {
		e.types = make(map[string]reflect.Type)
	}
	e.types[symbol] = typ
	return nil
}

// DefineGlobalType defines type in global scope.
func (e *Env) DefineGlobalType(symbol string, typ interface{}) error {
	for e.parent != nil {
		return e.parent.DefineType(symbol, typ)
	}
	return e.DefineType(symbol, typ)
}

// DefineGlobalReflectType defines type in global scope.
func (e *Env) DefineGlobalReflectType(symbol string, typ reflect.Type) error {
	for e.parent != nil {
		return e.parent.DefineReflectType(symbol, typ)
	}
	return e.DefineReflectType(symbol, typ)
}

// Type returns reflect type from the scope where symbol is first found.
func (e *Env) Type(symbol string) (reflect.Type, error) {
	e.rwm.RLock()
	defer e.rwm.RUnlock()
	typ, ok := e.types[symbol]
	if ok {
		return typ, nil
	}
	if e.extLookup != nil {
		var err error
		typ, err = e.extLookup.Type(symbol)
		if err == nil {
			return typ, nil
		}
	}
	if e.parent == nil {
		typ, ok = basicTypes[symbol]
		if ok {
			return typ, nil
		}
		return NilType, fmt.Errorf("undefined type '%s'", symbol)
	}
	return e.parent.Type(symbol)
}

// Types returns all Types in Env.
func (e *Env) Types() map[string]reflect.Type {
	e.rwm.RLock()
	defer e.rwm.RUnlock()
	types := make(map[string]reflect.Type, len(e.types))
	for symbol, typ := range e.types {
		types[symbol] = typ
	}
	return types
}
