package env

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Define defines/sets interface value to symbol in current scope.
func (e *Env) Define(symbol string, value interface{}) error {
	if value == nil {
		return e.DefineValue(symbol, NilValue)
	}
	return e.DefineValue(symbol, reflect.ValueOf(value))
}

// DefineValue defines/sets reflect value to symbol in current scope.
func (e *Env) DefineValue(symbol string, value reflect.Value) error {
	if strings.Contains(symbol, ".") {
		return ErrSymbolContainsDot
	}
	e.rwm.Lock()
	defer e.rwm.Unlock()
	e.values[symbol] = value
	return nil
}

// DefineGlobal defines/sets interface value to symbol in global scope.
func (e *Env) DefineGlobal(symbol string, value interface{}) error {
	for e.parent != nil {
		return e.parent.Define(symbol, value)
	}
	return e.Define(symbol, value)
}

// DefineGlobalValue defines/sets reflect value to symbol in global scope.
func (e *Env) DefineGlobalValue(symbol string, value reflect.Value) error {
	for e.parent != nil {
		return e.parent.DefineValue(symbol, value)
	}
	return e.DefineValue(symbol, value)
}

// Set interface value to the scope where symbol is first found.
func (e *Env) Set(symbol string, value interface{}) error {
	if value == nil {
		return e.SetValue(symbol, NilValue)
	}
	return e.SetValue(symbol, reflect.ValueOf(value))
}

// SetValue reflect value to the scope where symbol is first found.
func (e *Env) SetValue(symbol string, value reflect.Value) error {
	e.rwm.Lock()
	defer e.rwm.Unlock()
	_, ok := e.values[symbol]
	if ok {
		e.values[symbol] = value
		return nil
	}
	if e.parent == nil {
		return fmt.Errorf("undefined symbol \"%s\"", symbol)
	}
	return e.parent.SetValue(symbol, value)
}

// Get returns interface value from the scope where symbol is first found.
func (e *Env) Get(symbol string) (interface{}, error) {
	rv, err := e.GetValue(symbol)
	if err != nil {
		return nil, err
	}
	return rv.Interface(), err
}

// GetValue returns reflect value from the scope where symbol is first found.
func (e *Env) GetValue(symbol string) (reflect.Value, error) {
	e.rwm.RLock()
	defer e.rwm.RUnlock()
	value, ok := e.values[symbol]
	if ok {
		return value, nil
	}
	if e.extLookup != nil {
		var err error
		value, err = e.extLookup.Get(symbol)
		if err == nil {
			return value, nil
		}
	}
	if e.parent == nil {
		return NilValue, fmt.Errorf("undefined symbol \"%s\"", symbol)
	}
	return e.parent.GetValue(symbol)
}

// Delete deletes symbol in current scope.
func (e *Env) Delete(symbol string) {
	e.rwm.Lock()
	defer e.rwm.Unlock()
	delete(e.values, symbol)
}

// DeleteGlobal deletes the first matching symbol found in current or parent scope.
func (e *Env) DeleteGlobal(symbol string) {
	if e.parent == nil {
		e.Delete(symbol)
		return
	}
	e.rwm.Lock()
	defer e.rwm.Unlock()
	_, ok := e.values[symbol]
	if ok {
		delete(e.values, symbol)
		return
	}
	e.parent.DeleteGlobal(symbol)
}

// Addr returns reflect.Addr of value for first matching symbol found in current or parent scope.
func (e *Env) Addr(symbol string) (reflect.Value, error) {
	e.rwm.RLock()
	defer e.rwm.RUnlock()
	if v, ok := e.values[symbol]; ok {
		if v.CanAddr() {
			return v.Addr(), nil
		}
		return NilValue, errors.New("unaddressable")
	}
	if e.extLookup != nil {
		v, err := e.extLookup.Get(symbol)
		if err == nil {
			if v.CanAddr() {
				return v.Addr(), nil
			}
			return NilValue, errors.New("unaddressable")
		}
	}
	if e.parent == nil {
		return NilValue, fmt.Errorf("undefined symbol \"%s\"", symbol)
	}
	return e.parent.Addr(symbol)
}

// Values returns all values in Env.
func (e *Env) Values() map[string]reflect.Value {
	e.rwm.RLock()
	defer e.rwm.RUnlock()
	values := make(map[string]reflect.Value, len(e.values))
	for symbol, value := range e.values {
		values[symbol] = value
	}
	return values
}
