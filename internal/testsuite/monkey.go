package testsuite

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/bouk/monkey"
	"github.com/stretchr/testify/require"
)

// ErrMonkey is used to return an error in patch function
// see TestIsMonkeyError()
var ErrMonkey = errors.New("monkey error")

// IsMonkeyError is used to confirm err is ErrMonkey
func IsMonkeyError(t testing.TB, err error) {
	require.Equal(t, ErrMonkey, err)
}

// Patch is a wrapper about monkey.Patch
func Patch(target, replacement interface{}) *monkey.PatchGuard {
	return monkey.Patch(target, replacement)
}

// PatchInstanceMethod is used to PatchInstanceMethod if target is private structure
// see ExamplePatchInstanceMethod()
func PatchInstanceMethod(target reflect.Type, method string, replacement interface{}) *monkey.PatchGuard {
	m, ok := target.MethodByName(method)
	if !ok {
		panic(fmt.Sprintf("unknown method %s", method))
	}

	replacementInputLen := reflect.TypeOf(replacement).NumIn()
	if replacementInputLen > m.Type.NumIn() {
		const format = "replacement function has too many input parameters: %d, replaced function: %d"
		panic(fmt.Sprintf(format, replacementInputLen, m.Type.NumIn()))
	}

	replacementWrapper := reflect.MakeFunc(m.Type, func(args []reflect.Value) []reflect.Value {
		inputsForReplacement := make([]reflect.Value, 0, replacementInputLen)
		for i := 0; i < cap(inputsForReplacement); i++ {
			elem := args[i].Convert(reflect.TypeOf(replacement).In(i))
			inputsForReplacement = append(inputsForReplacement, elem)
		}
		return reflect.ValueOf(replacement).Call(inputsForReplacement)
	}).Interface()

	return monkey.PatchInstanceMethod(target, method, replacementWrapper)
}
