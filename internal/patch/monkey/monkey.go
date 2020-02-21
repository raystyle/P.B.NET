package monkey

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/bouk/monkey"
	"github.com/stretchr/testify/require"
)

// PatchGuard is a type alias.
type PatchGuard = monkey.PatchGuard

// ErrMonkey is used to return an error in patch function.
var ErrMonkey = errors.New("monkey error")

// IsMonkeyError is used to confirm err is ErrMonkey.
func IsMonkeyError(t testing.TB, err error) {
	require.Equal(t, ErrMonkey, err)
}

// Patch is a wrapper about monkey.Patch.
func Patch(target, replacement interface{}) *PatchGuard {
	return monkey.Patch(target, replacement)
}

// PatchInstanceMethod will add reflect.TypeOf(target).
func PatchInstanceMethod(target interface{}, method string, replacement interface{}) *PatchGuard {
	return PatchInstanceMethodType(reflect.TypeOf(target), method, replacement)
}

// PatchInstanceMethodType is used to PatchInstanceMethod if target is private structure.
func PatchInstanceMethodType(target reflect.Type, method string, replacement interface{}) *PatchGuard {
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
