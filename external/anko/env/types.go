package env

import (
	"reflect"
)

// Types returns all Types in Env.
func (e *Env) Types() map[string]reflect.Type {
	e.rwm.RLock()
	defer e.rwm.RUnlock()
	types := make(map[string]reflect.Type, len(e.types))
	for symbol, aType := range e.types {
		types[symbol] = aType
	}
	return types
}
