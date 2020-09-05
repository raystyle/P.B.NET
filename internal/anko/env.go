package anko

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"

	"github.com/mattn/anko/core"
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

func newRuntime() *runtime {
	return &runtime{
		values: make(map[string]reflect.Value),
		types:  make(map[string]reflect.Type),
	}
}

func (rt *runtime) Get(symbol string) (reflect.Value, error) {
	rt.valuesRWM.RLock()
	defer rt.valuesRWM.RUnlock()
	if value, ok := rt.values[symbol]; ok {
		return value, nil
	}
	return reflect.Value{}, fmt.Errorf("value %q is not defined", symbol)
}

func (rt *runtime) Set(symbol string, value interface{}) error {
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

func (rt *runtime) Type(symbol string) (reflect.Type, error) {
	rt.typesRWM.RLock()
	defer rt.typesRWM.RUnlock()
	if typ, ok := rt.types[symbol]; ok {
		return typ, nil
	}
	return nil, fmt.Errorf("type %q is not defined", symbol)
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
	Env     *env.Env
	runtime *runtime

	output io.Writer

	ctx    context.Context
	cancel context.CancelFunc
}

func newEnv(output io.Writer) *Env {
	e := env.NewEnv()
	core.ImportToX(e)
	defineConvert(e)
	defineCore(e)
	r := newRuntime()
	e.SetExternalLookup(r)
	en := &Env{
		Env:     e,
		runtime: r,
		output:  output,
	}
	en.ctx, en.cancel = context.WithCancel(context.Background())
	defineBuiltin(en)
	return en
}

func defineBuiltin(e *Env) {
	for _, item := range []*struct {
		symbol string
		fn     interface{}
	}{
		{"printf", e.printf},
		{"print", e.print},
		{"println", e.println},
		{"set", e.setGlobal},
		{"get", e.getGlobal},
		{"eval", e.eval},
		{"hello", e.hello},
	} {
		err := e.runtime.Set(item.symbol, item.fn)
		if err != nil {
			panic(fmt.Sprintf("anko: internal error: %s", err))
		}

		// fmt.Println(reflect.ValueOf(item.fn))
		//
		// fmt.Println(item.fn)
	}
}

func (e *Env) newEnv() *Env {
	en := &Env{
		Env:     e.Env.NewEnv(),
		runtime: e.runtime,
		output:  e.output,
	}
	en.ctx, en.cancel = context.WithCancel(e.ctx)
	return en
}

func (e *Env) importPackage() {
	fmt.Println("import!")
}

func (e *Env) printf(format string, v ...interface{}) {
	_, _ = fmt.Fprintf(e.output, format, v...)
}

func (e *Env) print(v ...interface{}) {
	_, _ = fmt.Fprint(e.output, v...)
}

func (e *Env) println(v ...interface{}) {
	_, _ = fmt.Fprintln(e.output, v...)
}

func (e *Env) setGlobal(symbol string, value interface{}) {
	e.runtime.Set("_global_"+symbol, value)
}

func (e *Env) getGlobal(symbol string) interface{} {
	val, err := e.runtime.Get("_global_" + symbol)
	if err != nil {
		return nil
	}
	return val
}

func (e *Env) hello() {
	fmt.Println("hello")
}

func (e *Env) eval(src string) (interface{}, error) {
	stmt, err := ParseSrc(src)
	if err != nil {
		return nil, err
	}
	en := e.newEnv()
	defer en.Close()
	val, err := RunContext(e.ctx, nil, stmt)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// SetOutput is used to set output for printf, print and println.
func (e *Env) SetOutput(output io.Writer) {
	e.output = output
}

// Close is used to close env and delete functions that reference self.
func (e *Env) Close() {
	e.cancel()
	for _, symbol := range [...]string{
		"printf",
		"print",
		"println",
		"set",
		"get",
		"eval",

		"hello",
	} {

		e.runtime.Set(symbol, nil)
	}

	// e.runtime.values = make(map[string]reflect.Value)

}
